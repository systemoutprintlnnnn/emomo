package repository

import (
	"context"
	"fmt"

	"github.com/timmy/emomo/internal/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// MemeRepository handles meme data operations.
type MemeRepository struct {
	db *gorm.DB
}

// NewMemeRepository creates a new MemeRepository.
// Parameters:
//   - db: GORM database handle used for queries.
// Returns:
//   - *MemeRepository: repository instance bound to db.
func NewMemeRepository(db *gorm.DB) *MemeRepository {
	return &MemeRepository{db: db}
}

// Create inserts a new meme record.
// Parameters:
//   - ctx: context for cancellation and deadlines.
//   - meme: meme record to persist.
// Returns:
//   - error: non-nil if the insert fails.
func (r *MemeRepository) Create(ctx context.Context, meme *domain.Meme) error {
	return r.db.WithContext(ctx).Create(meme).Error
}

// Upsert creates or updates a meme record keyed by source fields.
// Parameters:
//   - ctx: context for cancellation and deadlines.
//   - meme: meme record to create or update.
// Returns:
//   - error: non-nil if the upsert fails.
func (r *MemeRepository) Upsert(ctx context.Context, meme *domain.Meme) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "source_type"}, {Name: "source_id"}},
		UpdateAll: true,
	}).Create(meme).Error
}

// Update updates an existing meme record.
// Parameters:
//   - ctx: context for cancellation and deadlines.
//   - meme: meme record with updated fields.
// Returns:
//   - error: non-nil if the update fails.
func (r *MemeRepository) Update(ctx context.Context, meme *domain.Meme) error {
	return r.db.WithContext(ctx).Save(meme).Error
}

// GetByID retrieves a meme by its ID.
// Parameters:
//   - ctx: context for cancellation and deadlines.
//   - id: meme ID.
// Returns:
//   - *domain.Meme: meme record if found.
//   - error: non-nil if lookup fails.
func (r *MemeRepository) GetByID(ctx context.Context, id string) (*domain.Meme, error) {
	var meme domain.Meme
	if err := r.db.WithContext(ctx).First(&meme, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &meme, nil
}

// GetByMD5Hash retrieves a meme by its MD5 hash for deduplication.
// Parameters:
//   - ctx: context for cancellation and deadlines.
//   - md5Hash: MD5 hash of the meme content.
// Returns:
//   - *domain.Meme: meme record if found.
//   - error: non-nil if lookup fails.
func (r *MemeRepository) GetByMD5Hash(ctx context.Context, md5Hash string) (*domain.Meme, error) {
	var meme domain.Meme
	if err := r.db.WithContext(ctx).First(&meme, "md5_hash = ?", md5Hash).Error; err != nil {
		return nil, err
	}
	return &meme, nil
}

// ExistsByMD5Hash checks if a meme with the given MD5 hash exists.
// Parameters:
//   - ctx: context for cancellation and deadlines.
//   - md5Hash: MD5 hash of the meme content.
// Returns:
//   - bool: true if a record exists.
//   - error: non-nil if the lookup fails.
func (r *MemeRepository) ExistsByMD5Hash(ctx context.Context, md5Hash string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&domain.Meme{}).Where("md5_hash = ?", md5Hash).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetBySourceID retrieves a meme by source type and source ID.
// Parameters:
//   - ctx: context for cancellation and deadlines.
//   - sourceType: source type identifier.
//   - sourceID: source-specific ID.
// Returns:
//   - *domain.Meme: meme record if found.
//   - error: non-nil if lookup fails.
func (r *MemeRepository) GetBySourceID(ctx context.Context, sourceType, sourceID string) (*domain.Meme, error) {
	var meme domain.Meme
	if err := r.db.WithContext(ctx).First(&meme, "source_type = ? AND source_id = ?", sourceType, sourceID).Error; err != nil {
		return nil, err
	}
	return &meme, nil
}

// ExistsBySourceID checks if a meme exists by source type and source ID.
// Parameters:
//   - ctx: context for cancellation and deadlines.
//   - sourceType: source type identifier.
//   - sourceID: source-specific ID.
// Returns:
//   - bool: true if a record exists.
//   - error: non-nil if the lookup fails.
func (r *MemeRepository) ExistsBySourceID(ctx context.Context, sourceType, sourceID string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&domain.Meme{}).
		Where("source_type = ? AND source_id = ?", sourceType, sourceID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// ListByStatus retrieves memes by status with pagination.
// Parameters:
//   - ctx: context for cancellation and deadlines.
//   - status: meme status to filter by.
//   - limit: maximum number of records to return.
//   - offset: number of records to skip.
// Returns:
//   - []domain.Meme: matching meme records.
//   - error: non-nil if the query fails.
func (r *MemeRepository) ListByStatus(ctx context.Context, status domain.MemeStatus, limit, offset int) ([]domain.Meme, error) {
	var memes []domain.Meme
	if err := r.db.WithContext(ctx).
		Where("status = ?", status).
		Limit(limit).
		Offset(offset).
		Find(&memes).Error; err != nil {
		return nil, err
	}
	return memes, nil
}

// ListByCategory retrieves memes by category with pagination.
// Parameters:
//   - ctx: context for cancellation and deadlines.
//   - category: category name to filter by; empty means all.
//   - limit: maximum number of records to return.
//   - offset: number of records to skip.
// Returns:
//   - []domain.Meme: matching meme records.
//   - error: non-nil if the query fails.
func (r *MemeRepository) ListByCategory(ctx context.Context, category string, limit, offset int) ([]domain.Meme, error) {
	var memes []domain.Meme
	query := r.db.WithContext(ctx)
	if category != "" {
		query = query.Where("category = ?", category)
	}
	if err := query.
		Where("status = ?", domain.MemeStatusActive).
		Limit(limit).
		Offset(offset).
		Order("created_at DESC").
		Find(&memes).Error; err != nil {
		return nil, err
	}
	return memes, nil
}

// GetCategories retrieves all unique categories.
// Parameters:
//   - ctx: context for cancellation and deadlines.
// Returns:
//   - []string: distinct category names.
//   - error: non-nil if the query fails.
func (r *MemeRepository) GetCategories(ctx context.Context) ([]string, error) {
	var categories []string
	if err := r.db.WithContext(ctx).
		Model(&domain.Meme{}).
		Where("status = ?", domain.MemeStatusActive).
		Distinct("category").
		Pluck("category", &categories).Error; err != nil {
		return nil, err
	}
	return categories, nil
}

// CountByStatus counts memes by status.
// Parameters:
//   - ctx: context for cancellation and deadlines.
//   - status: meme status to count.
// Returns:
//   - int64: number of matching records.
//   - error: non-nil if the query fails.
func (r *MemeRepository) CountByStatus(ctx context.Context, status domain.MemeStatus) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&domain.Meme{}).Where("status = ?", status).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// GetByIDs retrieves memes by a list of IDs.
// Parameters:
//   - ctx: context for cancellation and deadlines.
//   - ids: list of meme IDs.
// Returns:
//   - []domain.Meme: matching meme records.
//   - error: non-nil if the query fails.
func (r *MemeRepository) GetByIDs(ctx context.Context, ids []string) ([]domain.Meme, error) {
	if len(ids) == 0 {
		return []domain.Meme{}, nil
	}
	var memes []domain.Meme
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&memes).Error; err != nil {
		return nil, fmt.Errorf("failed to get memes by IDs: %w", err)
	}
	return memes, nil
}

// Delete removes a meme by ID.
// Parameters:
//   - ctx: context for cancellation and deadlines.
//   - id: meme ID to delete.
// Returns:
//   - error: non-nil if the delete fails.
func (r *MemeRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&domain.Meme{}, "id = ?", id).Error
}
