package service

import (
	"context"
	"fmt"
	"strings"

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
	memeRepo           *repository.MemeRepository
	memeDescRepo       *repository.MemeDescriptionRepository
	defaultQdrantRepo  *repository.QdrantRepository
	defaultEmbedding   EmbeddingProvider
	queryUnderstanding *QueryUnderstandingService
	storage            storage.ObjectStorage
	logger             *logger.Logger
	scoreThreshold     float32
	defaultCollection  string

	// Multi-collection support: collection name -> config
	collections map[string]*CollectionConfig
}

// NewSearchService creates a new search service.
// Parameters:
//   - memeRepo: repository for meme records.
//   - memeDescRepo: repository for meme descriptions (metadata access).
//   - qdrantRepo: default Qdrant repository.
//   - embedding: default embedding provider.
//   - queryUnderstanding: optional query understanding service.
//   - objectStorage: object storage client for URL generation.
//   - log: logger instance.
//   - cfg: search configuration settings.
//
// Returns:
//   - *SearchService: initialized search service.
func NewSearchService(
	memeRepo *repository.MemeRepository,
	memeDescRepo *repository.MemeDescriptionRepository,
	qdrantRepo *repository.QdrantRepository,
	embedding EmbeddingProvider,
	queryUnderstanding *QueryUnderstandingService,
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
		memeRepo:           memeRepo,
		memeDescRepo:       memeDescRepo,
		defaultQdrantRepo:  qdrantRepo,
		defaultEmbedding:   embedding,
		queryUnderstanding: queryUnderstanding,
		storage:            objectStorage,
		logger:             log,
		scoreThreshold:     threshold,
		defaultCollection:  defaultCollection,
		collections:        make(map[string]*CollectionConfig),
	}
}

// RegisterCollection registers a collection configuration for multi-collection search.
// Parameters:
//   - name: collection name key.
//   - qdrantRepo: Qdrant repository for the collection.
//   - embedding: embedding provider for the collection.
//
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

// TextSearch performs a hybrid text search (dense + BM25).
// Parameters:
//   - ctx: context for cancellation and deadlines.
//   - req: search request parameters.
//
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

	// Inject search tracing fields into context
	ctx = logger.WithFields(ctx, logger.Fields{
		logger.FieldComponent: "search",
		logger.FieldSearchID:  fmt.Sprintf("%d", ctx.Value("request_id")), // Will be overwritten if request_id exists
	})

	// Use Query Understanding to analyze the query
	var queryPlan *QueryPlan
	if s.queryUnderstanding != nil && s.queryUnderstanding.IsEnabled() {
		var err error
		queryPlan, err = s.queryUnderstanding.Understand(ctx, originalQuery)
		if err != nil {
			logger.CtxWarn(ctx, "Query understanding failed: query=%q, error=%v", req.Query, err)
		}
	}
	if queryPlan == nil {
		// Fallback: use original query directly
		queryPlan = &QueryPlan{
			Intent:        IntentSemantic,
			SemanticQuery: originalQuery,
			Keywords:      []string{originalQuery},
			Strategy: SearchStrategy{
				DenseWeight:    0.7,
				NeedExactMatch: false,
			},
		}
	}

	// Use semantic query for embedding
	queryForEmbedding := queryPlan.SemanticQuery
	if queryForEmbedding == "" {
		queryForEmbedding = originalQuery
	}

	logger.CtxInfo(ctx, "Performing text search: query=%q, semantic_query=%q, intent=%s, top_k=%d, collection=%s",
		originalQuery, queryForEmbedding, queryPlan.Intent, req.TopK, collectionName)

	// Generate query embedding using the appropriate embedding provider
	queryEmbedding, err := embedding.EmbedQuery(ctx, queryForEmbedding)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	// Build filters - merge QueryPlan suggestions with request parameters (request takes priority)
	filters := &repository.SearchFilters{
		Category:   req.Category,
		IsAnimated: req.IsAnimated,
		SourceType: req.SourceType,
	}
	// Apply suggested filters if not overridden by request
	if queryPlan.Filters != nil {
		if req.Category == nil && len(queryPlan.Filters.Categories) > 0 {
			filters.Category = &queryPlan.Filters.Categories[0]
		}
		if req.IsAnimated == nil && queryPlan.Filters.IsAnimated != nil {
			filters.IsAnimated = queryPlan.Filters.IsAnimated
		}
	}

	// Build hybrid search plan from QueryPlan
	plan := buildHybridPlanFromQueryPlan(queryPlan, req.TopK)
	usingHybrid := true

	// Build BM25 query from keywords and synonyms
	bm25Query := buildBM25QueryFromPlan(queryPlan)

	qdrantResults, err := qdrantRepo.HybridSearch(ctx, queryEmbedding, bm25Query, req.TopK, &plan, filters)
	if err != nil {
		usingHybrid = false
		logger.CtxWarn(ctx, "Hybrid search failed, falling back to dense search: error=%v", err)
		qdrantResults, err = qdrantRepo.Search(ctx, queryEmbedding, req.TopK, filters)
		if err != nil {
			return nil, fmt.Errorf("failed to search in Qdrant: %w", err)
		}
	}

	results := make([]SearchResult, 0, req.TopK)
	for _, qr := range qdrantResults {
		if qr.Payload == nil {
			continue
		}
		if !usingHybrid && s.scoreThreshold > 0 && qr.Score < s.scoreThreshold {
			continue
		}
		results = append(results, SearchResult{
			ID:          qr.Payload.MemeID,
			URL:         qr.Payload.StorageURL,
			Score:       qr.Score,
			Description: qr.Payload.VLMDescription,
			Category:    qr.Payload.Category,
			Tags:        qr.Payload.Tags,
			IsAnimated:  qr.Payload.IsAnimated,
		})
	}

	// Slice to TopK
	if len(results) > req.TopK {
		results = results[:req.TopK]
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

	// Use semantic query as expanded query for backward compatibility
	expandedQuery := ""
	if queryPlan.SemanticQuery != originalQuery {
		expandedQuery = queryPlan.SemanticQuery
	}

	return &SearchResponse{
		Results:       results,
		Total:         len(results),
		Query:         originalQuery,
		ExpandedQuery: expandedQuery,
		Collection:    collectionName,
	}, nil
}

// TextSearchWithProgress performs a hybrid text search with progress updates.
// Parameters:
//   - ctx: context for cancellation and deadlines.
//   - req: search request parameters.
//   - progressCh: channel for sending progress updates.
//
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

	// Stage 1: Query Understanding (with streaming)
	var queryPlan *QueryPlan
	if s.queryUnderstanding != nil && s.queryUnderstanding.IsEnabled() {
		// Create progress channel for streaming
		understandProgressCh := make(chan UnderstandProgress, 100)
		understandDone := make(chan struct{})
		var understandErr error

		go func() {
			defer close(understandDone)
			queryPlan, understandErr = s.queryUnderstanding.UnderstandStream(ctx, originalQuery, understandProgressCh)
		}()

		// Forward understanding progress to search progress
		for progress := range understandProgressCh {
			switch progress.Stage {
			case "thinking_start":
				progressCh <- SearchProgress{
					Stage:   "query_expansion_start",
					Message: progress.Message,
				}
			case "thinking":
				progressCh <- SearchProgress{
					Stage:        "thinking",
					ThinkingText: progress.ThinkingText,
					IsDelta:      progress.IsDelta,
				}
			case "done":
				if queryPlan != nil && queryPlan.SemanticQuery != originalQuery {
					progressCh <- SearchProgress{
						Stage:         "query_expansion_done",
						Message:       "理解完成",
						ExpandedQuery: queryPlan.SemanticQuery,
					}
				}
			}
		}

		<-understandDone

		if understandErr != nil {
			logger.CtxWarn(ctx, "Query understanding failed: query=%q, error=%v",
				req.Query, understandErr)
		}
	}

	// Fallback if no query plan
	if queryPlan == nil {
		queryPlan = &QueryPlan{
			Intent:        IntentSemantic,
			SemanticQuery: originalQuery,
			Keywords:      []string{originalQuery},
			Strategy: SearchStrategy{
				DenseWeight:    0.7,
				NeedExactMatch: false,
			},
		}
	}

	// Use semantic query for embedding
	queryForEmbedding := queryPlan.SemanticQuery
	if queryForEmbedding == "" {
		queryForEmbedding = originalQuery
	}

	// Stage 2: Generate Embedding
	progressCh <- SearchProgress{
		Stage:   "embedding",
		Message: "正在生成语义向量...",
	}

	logger.CtxInfo(ctx, "Performing text search: query=%q, semantic_query=%q, intent=%s, top_k=%d, collection=%s",
		originalQuery, queryForEmbedding, queryPlan.Intent, req.TopK, collectionName)

	queryEmbedding, err := embedding.EmbedQuery(ctx, queryForEmbedding)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	// Stage 3: Search in Qdrant
	progressCh <- SearchProgress{
		Stage:   "searching",
		Message: "在表情库中搜索...",
	}

	// Build filters - merge QueryPlan suggestions with request parameters
	filters := &repository.SearchFilters{
		Category:   req.Category,
		IsAnimated: req.IsAnimated,
		SourceType: req.SourceType,
	}
	if queryPlan.Filters != nil {
		if req.Category == nil && len(queryPlan.Filters.Categories) > 0 {
			filters.Category = &queryPlan.Filters.Categories[0]
		}
		if req.IsAnimated == nil && queryPlan.Filters.IsAnimated != nil {
			filters.IsAnimated = queryPlan.Filters.IsAnimated
		}
	}

	// Build hybrid search plan from QueryPlan
	plan := buildHybridPlanFromQueryPlan(queryPlan, req.TopK)
	usingHybrid := true

	// Build BM25 query from keywords and synonyms
	bm25Query := buildBM25QueryFromPlan(queryPlan)

	qdrantResults, err := qdrantRepo.HybridSearch(ctx, queryEmbedding, bm25Query, req.TopK, &plan, filters)
	if err != nil {
		usingHybrid = false
		logger.CtxWarn(ctx, "Hybrid search failed, falling back to dense search: error=%v", err)
		progressCh <- SearchProgress{
			Stage:   "searching",
			Message: "混合检索失败，切换为语义检索...",
		}
		qdrantResults, err = qdrantRepo.Search(ctx, queryEmbedding, req.TopK, filters)
		if err != nil {
			return nil, fmt.Errorf("failed to search in Qdrant: %w", err)
		}
	}

	results := make([]SearchResult, 0, req.TopK)
	for _, qr := range qdrantResults {
		if qr.Payload == nil {
			continue
		}
		if !usingHybrid && s.scoreThreshold > 0 && qr.Score < s.scoreThreshold {
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

	// Slice to TopK
	if len(results) > req.TopK {
		results = results[:req.TopK]
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

	// Use semantic query as expanded query for backward compatibility
	expandedQuery := ""
	if queryPlan.SemanticQuery != originalQuery {
		expandedQuery = queryPlan.SemanticQuery
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
//
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
//
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
//
// Returns:
//   - *MemeListResponse: list results in search-compatible format.
//   - error: non-nil if retrieval fails.
//
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
			Score:       0,  // No score for listing (not a search)
			Description: "", // VLM description moved to meme_descriptions table; use search for descriptions
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
//
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

// buildHybridPlanFromQueryPlan builds a HybridSearchPlan from a QueryPlan.
// The mapping follows the design document section 5.1:
//   - dense_weight: 0.8 → DenseLimit: topK * 4, SparseLimit: topK * 1
//   - dense_weight: 0.5 → DenseLimit: topK * 2.5, SparseLimit: topK * 2.5
//   - dense_weight: 0.3 → DenseLimit: topK * 1.5, SparseLimit: topK * 3.5
func buildHybridPlanFromQueryPlan(queryPlan *QueryPlan, topK int) repository.HybridSearchPlan {
	if topK <= 0 {
		topK = 20
	}

	const maxPrefetch = 200
	const defaultRRFK = 60

	dw := queryPlan.Strategy.DenseWeight

	// Linear interpolation for prefetch limits based on dense_weight
	// dense_weight 0 → dense 1x, sparse 4x
	// dense_weight 1 → dense 4x, sparse 1x
	denseMultiplier := 1.0 + 3.0*float64(dw)  // 1 to 4
	sparseMultiplier := 4.0 - 3.0*float64(dw) // 4 to 1

	denseLimit := int(float64(topK) * denseMultiplier)
	sparseLimit := int(float64(topK) * sparseMultiplier)

	// Clamp to max prefetch
	if denseLimit > maxPrefetch {
		denseLimit = maxPrefetch
	}
	if sparseLimit > maxPrefetch {
		sparseLimit = maxPrefetch
	}
	if denseLimit < 1 {
		denseLimit = 1
	}
	if sparseLimit < 1 {
		sparseLimit = 1
	}

	return repository.HybridSearchPlan{
		UseDense:    true,
		UseSparse:   true,
		DenseLimit:  denseLimit,
		SparseLimit: sparseLimit,
		RRFK:        defaultRRFK,
	}
}

// buildBM25QueryFromPlan builds a BM25 query string from a QueryPlan.
// If need_exact_match is true, only keywords are used.
// Otherwise, keywords and synonyms are combined.
func buildBM25QueryFromPlan(queryPlan *QueryPlan) string {
	var terms []string

	// Always include keywords
	terms = append(terms, queryPlan.Keywords...)

	// Include synonyms if not exact match
	if !queryPlan.Strategy.NeedExactMatch {
		terms = append(terms, queryPlan.Synonyms...)
	}

	// Deduplicate and join
	seen := make(map[string]bool)
	var unique []string
	for _, term := range terms {
		lower := strings.ToLower(strings.TrimSpace(term))
		if lower != "" && !seen[lower] {
			seen[lower] = true
			unique = append(unique, term)
		}
	}

	return strings.Join(unique, " ")
}
