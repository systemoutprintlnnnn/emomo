package service

import (
	"context"
	"fmt"

	"github.com/timmy/emomo/internal/domain"
	"github.com/timmy/emomo/internal/logger"
	"github.com/timmy/emomo/internal/repository"
)

// SearchConfig holds configuration for search service
type SearchConfig struct {
	ScoreThreshold float32
}

// SearchService handles meme search operations
type SearchService struct {
	memeRepo       *repository.MemeRepository
	qdrantRepo     *repository.QdrantRepository
	embedding      *EmbeddingService
	queryExpansion *QueryExpansionService
	logger         *logger.Logger
	scoreThreshold float32
}

// NewSearchService creates a new search service
func NewSearchService(
	memeRepo *repository.MemeRepository,
	qdrantRepo *repository.QdrantRepository,
	embedding *EmbeddingService,
	queryExpansion *QueryExpansionService,
	log *logger.Logger,
	cfg *SearchConfig,
) *SearchService {
	var threshold float32
	if cfg != nil {
		threshold = cfg.ScoreThreshold
	}
	return &SearchService{
		memeRepo:       memeRepo,
		qdrantRepo:     qdrantRepo,
		embedding:      embedding,
		queryExpansion: queryExpansion,
		logger:         log,
		scoreThreshold: threshold,
	}
}

// log returns a logger from context if available, otherwise returns the default logger
func (s *SearchService) log(ctx context.Context) *logger.Logger {
	if l := logger.FromContext(ctx); l != nil {
		return l
	}
	return s.logger
}

// SearchRequest represents a text search request
type SearchRequest struct {
	Query      string  `json:"query" binding:"required"`
	TopK       int     `json:"top_k"`
	Category   *string `json:"category,omitempty"`
	IsAnimated *bool   `json:"is_animated,omitempty"`
	SourceType *string `json:"source_type,omitempty"`
}

// SearchResult represents a single search result
type SearchResult struct {
	ID             string   `json:"id"`
	URL            string   `json:"url"`
	Score          float32  `json:"score"`
	Description    string   `json:"description"`
	Category       string   `json:"category"`
	Tags           []string `json:"tags"`
	IsAnimated     bool     `json:"is_animated"`
	Width          int      `json:"width,omitempty"`
	Height         int      `json:"height,omitempty"`
}

// SearchResponse represents the search response
type SearchResponse struct {
	Results       []SearchResult `json:"results"`
	Total         int            `json:"total"`
	Query         string         `json:"query"`
	ExpandedQuery string         `json:"expanded_query,omitempty"`
}

// TextSearch performs a semantic text search
func (s *SearchService) TextSearch(ctx context.Context, req *SearchRequest) (*SearchResponse, error) {
	// Set defaults
	if req.TopK <= 0 {
		req.TopK = 20
	}
	if req.TopK > 100 {
		req.TopK = 100
	}

	originalQuery := req.Query
	expandedQuery := ""

	// Expand query using LLM if enabled
	if s.queryExpansion != nil && s.queryExpansion.IsEnabled() {
		expanded, err := s.queryExpansion.Expand(ctx, req.Query)
		if err != nil {
			s.log(ctx).WithFields(logger.Fields{
				"query": req.Query,
			}).WithError(err).Warn("Query expansion failed, using original query")
		} else if expanded != req.Query {
			expandedQuery = expanded
			s.log(ctx).WithFields(logger.Fields{
				"original": req.Query,
				"expanded": expanded,
			}).Info("Query expanded")
		}
	}

	// Use expanded query for embedding if available
	queryForEmbedding := originalQuery
	if expandedQuery != "" {
		queryForEmbedding = expandedQuery
	}

	s.log(ctx).WithFields(logger.Fields{
		"query":               originalQuery,
		"query_for_embedding": queryForEmbedding,
		"top_k":               req.TopK,
		"score_threshold":     s.scoreThreshold,
	}).Info("Performing text search")

	// Generate query embedding
	queryEmbedding, err := s.embedding.EmbedQuery(ctx, queryForEmbedding)
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
	qdrantResults, err := s.qdrantRepo.Search(ctx, queryEmbedding, req.TopK, filters)
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

	// Optionally enrich with full meme data from SQLite
	if len(results) > 0 {
		ids := make([]string, len(results))
		for i, r := range results {
			ids[i] = r.ID
		}

		memes, err := s.memeRepo.GetByIDs(ctx, ids)
		if err != nil {
			s.log(ctx).WithError(err).Warn("Failed to enrich results from database")
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
	}, nil
}

// GetCategories returns all available categories
func (s *SearchService) GetCategories(ctx context.Context) ([]string, error) {
	return s.memeRepo.GetCategories(ctx)
}

// GetMemeByID retrieves a meme by its ID
func (s *SearchService) GetMemeByID(ctx context.Context, id string) (*domain.Meme, error) {
	return s.memeRepo.GetByID(ctx, id)
}

// ListMemes retrieves memes with optional category filter
func (s *SearchService) ListMemes(ctx context.Context, category string, limit, offset int) ([]domain.Meme, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	return s.memeRepo.ListByCategory(ctx, category, limit, offset)
}

// GetStats returns search-related statistics
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
		"total_active":      activeCount,
		"total_pending":     pendingCount,
		"total_categories":  len(categories),
	}, nil
}
