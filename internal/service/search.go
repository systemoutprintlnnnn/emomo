package service

import (
	"context"
	"fmt"

	"github.com/timmy/emomo/internal/domain"
	"github.com/timmy/emomo/internal/logger"
	"github.com/timmy/emomo/internal/repository"
	"github.com/timmy/emomo/internal/storage"
)

// SearchConfig holds configuration for search service.
type SearchConfig struct {
	ScoreThreshold    float32
	DefaultCollection string // Default collection name for search
}

// CollectionConfig holds configuration for a single collection.
type CollectionConfig struct {
	QdrantRepo *repository.QdrantRepository
	Embedding  EmbeddingProvider
}

// SearchService handles meme search operations.
type SearchService struct {
	memeRepo          *repository.MemeRepository
	defaultQdrantRepo *repository.QdrantRepository
	defaultEmbedding  EmbeddingProvider
	queryExpansion    *QueryExpansionService
	storage           storage.ObjectStorage
	logger            *logger.Logger
	scoreThreshold    float32
	defaultCollection string

	// Multi-collection support: collection name -> config
	collections map[string]*CollectionConfig
}

// NewSearchService creates a new search service.
// Parameters:
//   - memeRepo: repository for meme records.
//   - qdrantRepo: default Qdrant repository.
//   - embedding: default embedding provider.
//   - queryExpansion: optional query expansion service.
//   - objectStorage: object storage client for URL generation.
//   - log: logger instance.
//   - cfg: search configuration settings.
// Returns:
//   - *SearchService: initialized search service.
func NewSearchService(
	memeRepo *repository.MemeRepository,
	qdrantRepo *repository.QdrantRepository,
	embedding EmbeddingProvider,
	queryExpansion *QueryExpansionService,
	objectStorage storage.ObjectStorage,
	log *logger.Logger,
	cfg *SearchConfig,
) *SearchService {
	var threshold float32
	var defaultCollection string
	if cfg != nil {
		threshold = cfg.ScoreThreshold
		defaultCollection = cfg.DefaultCollection
	}
	return &SearchService{
		memeRepo:          memeRepo,
		defaultQdrantRepo: qdrantRepo,
		defaultEmbedding:  embedding,
		queryExpansion:    queryExpansion,
		storage:           objectStorage,
		logger:            log,
		scoreThreshold:    threshold,
		defaultCollection: defaultCollection,
		collections:       make(map[string]*CollectionConfig),
	}
}

// RegisterCollection registers a collection configuration for multi-collection search.
// Parameters:
//   - name: collection name key.
//   - qdrantRepo: Qdrant repository for the collection.
//   - embedding: embedding provider for the collection.
// Returns: none.
func (s *SearchService) RegisterCollection(name string, qdrantRepo *repository.QdrantRepository, embedding EmbeddingProvider) {
	s.collections[name] = &CollectionConfig{
		QdrantRepo: qdrantRepo,
		Embedding:  embedding,
	}
}

// GetAvailableCollections returns the list of available collection names.
// Parameters: none.
// Returns:
//   - []string: collection names including default and registered ones.
func (s *SearchService) GetAvailableCollections() []string {
	collections := make([]string, 0, len(s.collections)+1)
	if s.defaultCollection != "" {
		collections = append(collections, s.defaultCollection)
	}
	for name := range s.collections {
		if name != s.defaultCollection {
			collections = append(collections, name)
		}
	}
	return collections
}

// log returns a logger from context if available, otherwise returns the default logger
func (s *SearchService) log(ctx context.Context) *logger.Logger {
	if l := logger.FromContext(ctx); l != nil {
		return l
	}
	return s.logger
}

// SearchRequest represents a text search request.
type SearchRequest struct {
	Query      string  `json:"query" binding:"required"`
	TopK       int     `json:"top_k"`
	Category   *string `json:"category,omitempty"`
	IsAnimated *bool   `json:"is_animated,omitempty"`
	SourceType *string `json:"source_type,omitempty"`
	Collection string  `json:"collection,omitempty"` // Optional: specify which collection to search
}

// SearchResult represents a single search result.
type SearchResult struct {
	ID          string   `json:"id"`
	URL         string   `json:"url"`
	Score       float32  `json:"score"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Tags        []string `json:"tags"`
	IsAnimated  bool     `json:"is_animated"`
	Width       int      `json:"width,omitempty"`
	Height      int      `json:"height,omitempty"`
}

// SearchResponse represents the search response.
type SearchResponse struct {
	Results       []SearchResult `json:"results"`
	Total         int            `json:"total"`
	Query         string         `json:"query"`
	ExpandedQuery string         `json:"expanded_query,omitempty"`
	Collection    string         `json:"collection,omitempty"` // Which collection was searched
}

// SearchProgress represents a progress update during streaming search.
type SearchProgress struct {
	Stage         string `json:"stage"`                    // Current stage of the search
	Message       string `json:"message,omitempty"`        // User-friendly message
	ThinkingText  string `json:"thinking_text,omitempty"`  // LLM thinking content (for streaming)
	IsDelta       bool   `json:"is_delta,omitempty"`       // Whether thinking_text is incremental
	ExpandedQuery string `json:"expanded_query,omitempty"` // Expanded query (when available)
}

// TextSearch performs a semantic text search.
// Parameters:
//   - ctx: context for cancellation and deadlines.
//   - req: search request parameters.
// Returns:
//   - *SearchResponse: search results and metadata.
//   - error: non-nil if search fails.
func (s *SearchService) TextSearch(ctx context.Context, req *SearchRequest) (*SearchResponse, error) {
	// Set defaults
	if req.TopK <= 0 {
		req.TopK = 20
	}
	if req.TopK > 100 {
		req.TopK = 100
	}

	// Determine which collection, embedding, and qdrant repo to use
	var qdrantRepo *repository.QdrantRepository
	var embedding EmbeddingProvider
	var collectionName string

	if req.Collection != "" {
		// Use specified collection
		if cfg, ok := s.collections[req.Collection]; ok {
			qdrantRepo = cfg.QdrantRepo
			embedding = cfg.Embedding
			collectionName = req.Collection
		} else {
			return nil, fmt.Errorf("unknown collection: %s", req.Collection)
		}
	} else {
		// Use default collection
		qdrantRepo = s.defaultQdrantRepo
		embedding = s.defaultEmbedding
		collectionName = s.defaultCollection
	}

	originalQuery := req.Query
	expandedQuery := ""

	// Inject search tracing fields into context
	ctx = logger.WithFields(ctx, logger.Fields{
		logger.FieldComponent: "search",
		logger.FieldSearchID:  fmt.Sprintf("%d", ctx.Value("request_id")), // Will be overwritten if request_id exists
	})

	// Expand query using LLM if enabled
	if s.queryExpansion != nil && s.queryExpansion.IsEnabled() {
		expanded, err := s.queryExpansion.Expand(ctx, req.Query)
		if err != nil {
			logger.CtxWarn(ctx, "Query expansion failed, using original query: query=%q, error=%v",
				req.Query, err)
		} else if expanded != req.Query {
			expandedQuery = expanded
			logger.CtxInfo(ctx, "Query expanded: original=%q, expanded=%q", req.Query, expanded)
		}
	}

	// Use expanded query for embedding if available
	queryForEmbedding := originalQuery
	if expandedQuery != "" {
		queryForEmbedding = expandedQuery
	}

	logger.CtxInfo(ctx, "Performing text search: query=%q, query_for_embedding=%q, top_k=%d, collection=%s",
		originalQuery, queryForEmbedding, req.TopK, collectionName)

	// Generate query embedding using the appropriate embedding provider
	queryEmbedding, err := embedding.EmbedQuery(ctx, queryForEmbedding)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	// Build filters
	filters := &repository.SearchFilters{
		Category:   req.Category,
		IsAnimated: req.IsAnimated,
		SourceType: req.SourceType,
	}

	// Search in Qdrant
	qdrantResults, err := qdrantRepo.Search(ctx, queryEmbedding, req.TopK, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to search in Qdrant: %w", err)
	}

	// Convert to response format, filtering by score threshold
	results := make([]SearchResult, 0, len(qdrantResults))
	for _, qr := range qdrantResults {
		if qr.Payload == nil {
			continue
		}

		// Filter out results below score threshold
		if s.scoreThreshold > 0 && qr.Score < s.scoreThreshold {
			continue
		}

		result := SearchResult{
			ID:          qr.Payload.MemeID,
			URL:         qr.Payload.StorageURL,
			Score:       qr.Score,
			Description: qr.Payload.VLMDescription,
			Category:    qr.Payload.Category,
			Tags:        qr.Payload.Tags,
			IsAnimated:  qr.Payload.IsAnimated,
		}

		results = append(results, result)
	}

	// Optionally enrich with full meme data from database
	if len(results) > 0 {
		ids := make([]string, len(results))
		for i, r := range results {
			ids[i] = r.ID
		}

		memes, err := s.memeRepo.GetByIDs(ctx, ids)
		if err != nil {
			logger.CtxWarn(ctx, "Failed to enrich results from database: error=%v", err)
		} else {
			memeMap := make(map[string]*domain.Meme)
			for i := range memes {
				memeMap[memes[i].ID] = &memes[i]
			}

			for i := range results {
				if meme, ok := memeMap[results[i].ID]; ok {
					results[i].Width = meme.Width
					results[i].Height = meme.Height
				}
			}
		}
	}

	return &SearchResponse{
		Results:       results,
		Total:         len(results),
		Query:         originalQuery,
		ExpandedQuery: expandedQuery,
		Collection:    collectionName,
	}, nil
}

// TextSearchWithProgress performs a semantic text search with progress updates.
// Parameters:
//   - ctx: context for cancellation and deadlines.
//   - req: search request parameters.
//   - progressCh: channel for sending progress updates.
// Returns:
//   - *SearchResponse: search results and metadata.
//   - error: non-nil if search fails.
func (s *SearchService) TextSearchWithProgress(ctx context.Context, req *SearchRequest, progressCh chan<- SearchProgress) (*SearchResponse, error) {
	defer close(progressCh)

	// Set defaults
	if req.TopK <= 0 {
		req.TopK = 20
	}
	if req.TopK > 100 {
		req.TopK = 100
	}

	// Determine which collection, embedding, and qdrant repo to use
	var qdrantRepo *repository.QdrantRepository
	var embedding EmbeddingProvider
	var collectionName string

	if req.Collection != "" {
		if cfg, ok := s.collections[req.Collection]; ok {
			qdrantRepo = cfg.QdrantRepo
			embedding = cfg.Embedding
			collectionName = req.Collection
		} else {
			return nil, fmt.Errorf("unknown collection: %s", req.Collection)
		}
	} else {
		qdrantRepo = s.defaultQdrantRepo
		embedding = s.defaultEmbedding
		collectionName = s.defaultCollection
	}

	originalQuery := req.Query
	expandedQuery := ""

	// Stage 1: Query Expansion (with streaming)
	if s.queryExpansion != nil && s.queryExpansion.IsEnabled() {
		// Send start event
		progressCh <- SearchProgress{
			Stage:   "query_expansion_start",
			Message: "AI 正在理解搜索意图...",
		}

		// Create token channel for streaming
		tokenCh := make(chan string, 100)
		expandDone := make(chan struct{})
		var expandErr error

		go func() {
			defer close(expandDone)
			expandedQuery, expandErr = s.queryExpansion.ExpandStream(ctx, req.Query, tokenCh)
		}()

		// Stream thinking tokens
		for token := range tokenCh {
			progressCh <- SearchProgress{
				Stage:        "thinking",
				ThinkingText: token,
				IsDelta:      true,
			}
		}

		<-expandDone

		if expandErr != nil {
			logger.CtxWarn(ctx, "Query expansion failed, using original query: query=%q, error=%v",
				req.Query, expandErr)
			// Silent fallback - continue with original query
			expandedQuery = ""
		} else if expandedQuery != req.Query && expandedQuery != "" {
			logger.CtxInfo(ctx, "Query expanded: original=%q, expanded=%q", req.Query, expandedQuery)

			progressCh <- SearchProgress{
				Stage:         "query_expansion_done",
				Message:       "理解完成",
				ExpandedQuery: expandedQuery,
			}
		}
	}

	// Use expanded query for embedding if available
	queryForEmbedding := originalQuery
	if expandedQuery != "" {
		queryForEmbedding = expandedQuery
	}

	// Stage 2: Generate Embedding
	progressCh <- SearchProgress{
		Stage:   "embedding",
		Message: "正在生成语义向量...",
	}

	logger.CtxInfo(ctx, "Performing text search: query=%q, query_for_embedding=%q, top_k=%d, collection=%s",
		originalQuery, queryForEmbedding, req.TopK, collectionName)

	queryEmbedding, err := embedding.EmbedQuery(ctx, queryForEmbedding)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	// Stage 3: Search in Qdrant
	progressCh <- SearchProgress{
		Stage:   "searching",
		Message: "在表情库中搜索...",
	}

	filters := &repository.SearchFilters{
		Category:   req.Category,
		IsAnimated: req.IsAnimated,
		SourceType: req.SourceType,
	}

	qdrantResults, err := qdrantRepo.Search(ctx, queryEmbedding, req.TopK, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to search in Qdrant: %w", err)
	}

	// Convert to response format
	results := make([]SearchResult, 0, len(qdrantResults))
	for _, qr := range qdrantResults {
		if qr.Payload == nil {
			continue
		}
		if s.scoreThreshold > 0 && qr.Score < s.scoreThreshold {
			continue
		}

		result := SearchResult{
			ID:          qr.Payload.MemeID,
			URL:         qr.Payload.StorageURL,
			Score:       qr.Score,
			Description: qr.Payload.VLMDescription,
			Category:    qr.Payload.Category,
			Tags:        qr.Payload.Tags,
			IsAnimated:  qr.Payload.IsAnimated,
		}
		results = append(results, result)
	}

	// Stage 4: Enrich with database data
	if len(results) > 0 {
		progressCh <- SearchProgress{
			Stage:   "enriching",
			Message: "加载表情包详情...",
		}

		ids := make([]string, len(results))
		for i, r := range results {
			ids[i] = r.ID
		}

		memes, err := s.memeRepo.GetByIDs(ctx, ids)
		if err != nil {
			logger.CtxWarn(ctx, "Failed to enrich results from database: error=%v", err)
		} else {
			memeMap := make(map[string]*domain.Meme)
			for i := range memes {
				memeMap[memes[i].ID] = &memes[i]
			}

			for i := range results {
				if meme, ok := memeMap[results[i].ID]; ok {
					results[i].Width = meme.Width
					results[i].Height = meme.Height
				}
			}
		}
	}

	return &SearchResponse{
		Results:       results,
		Total:         len(results),
		Query:         originalQuery,
		ExpandedQuery: expandedQuery,
		Collection:    collectionName,
	}, nil
}

// GetCategories returns all available categories.
// Parameters:
//   - ctx: context for cancellation and deadlines.
// Returns:
//   - []string: distinct category names.
//   - error: non-nil if lookup fails.
func (s *SearchService) GetCategories(ctx context.Context) ([]string, error) {
	return s.memeRepo.GetCategories(ctx)
}

// GetMemeByID retrieves a meme by its ID.
// Parameters:
//   - ctx: context for cancellation and deadlines.
//   - id: meme ID.
// Returns:
//   - *domain.Meme: meme record if found.
//   - error: non-nil if lookup fails.
func (s *SearchService) GetMemeByID(ctx context.Context, id string) (*domain.Meme, error) {
	return s.memeRepo.GetByID(ctx, id)
}

// MemeListResponse represents the response for listing memes.
type MemeListResponse struct {
	Results []SearchResult `json:"results"`
	Total   int            `json:"total"`
	Limit   int            `json:"limit"`
	Offset  int            `json:"offset"`
}

// ListMemes retrieves memes with optional category filter.
// Parameters:
//   - ctx: context for cancellation and deadlines.
//   - category: category name to filter by; empty means all.
//   - limit: maximum number of records to return.
//   - offset: number of records to skip.
// Returns:
//   - *MemeListResponse: list results in search-compatible format.
//   - error: non-nil if retrieval fails.
// Returns results in the same format as search results for API consistency.
func (s *SearchService) ListMemes(ctx context.Context, category string, limit, offset int) (*MemeListResponse, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	memes, err := s.memeRepo.ListByCategory(ctx, category, limit, offset)
	if err != nil {
		return nil, err
	}

	// Convert domain.Meme to SearchResult format for API consistency
	results := make([]SearchResult, len(memes))
	for i, meme := range memes {
		// Generate URL from storage_key
		url := ""
		if meme.StorageKey != "" && s.storage != nil {
			url = s.storage.GetURL(meme.StorageKey)
		}

		results[i] = SearchResult{
			ID:          meme.ID,
			URL:         url,
			Score:       0, // No score for listing (not a search)
			Description: meme.VLMDescription,
			Category:    meme.Category,
			Tags:        meme.Tags,
			IsAnimated:  meme.IsAnimated,
			Width:       meme.Width,
			Height:      meme.Height,
		}
	}

	return &MemeListResponse{
		Results: results,
		Total:   len(results),
		Limit:   limit,
		Offset:  offset,
	}, nil
}

// GetStats returns search-related statistics.
// Parameters:
//   - ctx: context for cancellation and deadlines.
// Returns:
//   - map[string]interface{}: aggregated stats for search and ingest.
//   - error: non-nil if statistics cannot be computed.
func (s *SearchService) GetStats(ctx context.Context) (map[string]interface{}, error) {
	activeCount, err := s.memeRepo.CountByStatus(ctx, domain.MemeStatusActive)
	if err != nil {
		return nil, err
	}

	pendingCount, err := s.memeRepo.CountByStatus(ctx, domain.MemeStatusPending)
	if err != nil {
		return nil, err
	}

	categories, err := s.memeRepo.GetCategories(ctx)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"total_active":          activeCount,
		"total_pending":         pendingCount,
		"total_categories":      len(categories),
		"available_collections": s.GetAvailableCollections(),
	}, nil
}
