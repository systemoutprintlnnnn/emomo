package repository

import (
	"context"

	"github.com/timmy/emomo/internal/domain"
	"gorm.io/gorm"
)

// MemeVectorRepository handles meme vector data operations
type MemeVectorRepository struct {
	db *gorm.DB
}

// NewMemeVectorRepository creates a new MemeVectorRepository
func NewMemeVectorRepository(db *gorm.DB) *MemeVectorRepository {
	return &MemeVectorRepository{db: db}
}

// Create creates a new meme vector record
func (r *MemeVectorRepository) Create(ctx context.Context, vector *domain.MemeVector) error {
	return r.db.WithContext(ctx).Create(vector).Error
}

// ExistsByMD5AndCollection checks if a vector record exists for the given MD5 hash and collection
func (r *MemeVectorRepository) ExistsByMD5AndCollection(ctx context.Context, md5Hash, collection string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&domain.MemeVector{}).
		Where("md5_hash = ? AND collection = ?", md5Hash, collection).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetByMD5AndCollection retrieves a vector record by MD5 hash and collection
func (r *MemeVectorRepository) GetByMD5AndCollection(ctx context.Context, md5Hash, collection string) (*domain.MemeVector, error) {
	var vector domain.MemeVector
	if err := r.db.WithContext(ctx).
		Where("md5_hash = ? AND collection = ?", md5Hash, collection).
		First(&vector).Error; err != nil {
		return nil, err
	}
	return &vector, nil
}

// GetByMemeID retrieves all vector records for a given meme ID
func (r *MemeVectorRepository) GetByMemeID(ctx context.Context, memeID string) ([]domain.MemeVector, error) {
	var vectors []domain.MemeVector
	if err := r.db.WithContext(ctx).
		Where("meme_id = ?", memeID).
		Find(&vectors).Error; err != nil {
		return nil, err
	}
	return vectors, nil
}

// GetByCollection retrieves all vector records for a given collection
func (r *MemeVectorRepository) GetByCollection(ctx context.Context, collection string, limit, offset int) ([]domain.MemeVector, error) {
	var vectors []domain.MemeVector
	query := r.db.WithContext(ctx).Where("collection = ?", collection)
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	if err := query.Find(&vectors).Error; err != nil {
		return nil, err
	}
	return vectors, nil
}

// CountByCollection counts the number of vectors in a collection
func (r *MemeVectorRepository) CountByCollection(ctx context.Context, collection string) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&domain.MemeVector{}).
		Where("collection = ?", collection).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// Delete deletes a meme vector by ID
func (r *MemeVectorRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&domain.MemeVector{}, "id = ?", id).Error
}

// DeleteByMemeIDAndCollection deletes a vector record by meme ID and collection
func (r *MemeVectorRepository) DeleteByMemeIDAndCollection(ctx context.Context, memeID, collection string) error {
	return r.db.WithContext(ctx).
		Where("meme_id = ? AND collection = ?", memeID, collection).
		Delete(&domain.MemeVector{}).Error
}

