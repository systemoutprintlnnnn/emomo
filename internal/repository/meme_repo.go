package repository

import (
	"context"
	"fmt"

	"github.com/timmy/emomo/internal/domain"
	"gorm.io/gorm"
)

// MemeRepository handles meme data operations
type MemeRepository struct {
	db *gorm.DB
}

// NewMemeRepository creates a new MemeRepository
func NewMemeRepository(db *gorm.DB) *MemeRepository {
	return &MemeRepository{db: db}
}

// Create creates a new meme record
func (r *MemeRepository) Create(ctx context.Context, meme *domain.Meme) error {
	return r.db.WithContext(ctx).Create(meme).Error
}

// Update updates an existing meme record
func (r *MemeRepository) Update(ctx context.Context, meme *domain.Meme) error {
	return r.db.WithContext(ctx).Save(meme).Error
}

// GetByID retrieves a meme by its ID
func (r *MemeRepository) GetByID(ctx context.Context, id string) (*domain.Meme, error) {
	var meme domain.Meme
	if err := r.db.WithContext(ctx).First(&meme, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &meme, nil
}

// GetByMD5Hash retrieves a meme by its MD5 hash for deduplication
func (r *MemeRepository) GetByMD5Hash(ctx context.Context, md5Hash string) (*domain.Meme, error) {
	var meme domain.Meme
	if err := r.db.WithContext(ctx).First(&meme, "md5_hash = ?", md5Hash).Error; err != nil {
		return nil, err
	}
	return &meme, nil
}

// ExistsByMD5Hash checks if a meme with the given MD5 hash exists
func (r *MemeRepository) ExistsByMD5Hash(ctx context.Context, md5Hash string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&domain.Meme{}).Where("md5_hash = ?", md5Hash).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetBySourceID retrieves a meme by source type and source ID
func (r *MemeRepository) GetBySourceID(ctx context.Context, sourceType, sourceID string) (*domain.Meme, error) {
	var meme domain.Meme
	if err := r.db.WithContext(ctx).First(&meme, "source_type = ? AND source_id = ?", sourceType, sourceID).Error; err != nil {
		return nil, err
	}
	return &meme, nil
}

// ExistsBySourceID checks if a meme exists by source type and source ID
func (r *MemeRepository) ExistsBySourceID(ctx context.Context, sourceType, sourceID string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&domain.Meme{}).
		Where("source_type = ? AND source_id = ?", sourceType, sourceID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// ListByStatus retrieves memes by status with pagination
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

// ListByCategory retrieves memes by category with pagination
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

// GetCategories retrieves all unique categories
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

// CountByStatus counts memes by status
func (r *MemeRepository) CountByStatus(ctx context.Context, status domain.MemeStatus) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&domain.Meme{}).Where("status = ?", status).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// GetByIDs retrieves memes by a list of IDs
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

// Delete deletes a meme by ID
func (r *MemeRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&domain.Meme{}, "id = ?", id).Error
}
