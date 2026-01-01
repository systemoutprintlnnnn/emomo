package repository

import (
	"context"

	"github.com/timmy/emomo/internal/domain"
	"gorm.io/gorm"
)

// MemeVectorRepository handles meme vector data operations.
type MemeVectorRepository struct {
	db *gorm.DB
}

// NewMemeVectorRepository creates a new MemeVectorRepository.
// Parameters:
//   - db: GORM database handle used for queries.
// Returns:
//   - *MemeVectorRepository: repository instance bound to db.
func NewMemeVectorRepository(db *gorm.DB) *MemeVectorRepository {
	return &MemeVectorRepository{db: db}
}

// Create inserts a new meme vector record.
// Parameters:
//   - ctx: context for cancellation and deadlines.
//   - vector: meme vector record to persist.
// Returns:
//   - error: non-nil if the insert fails.
func (r *MemeVectorRepository) Create(ctx context.Context, vector *domain.MemeVector) error {
	return r.db.WithContext(ctx).Create(vector).Error
}

// ExistsByMD5AndCollection checks if a vector record exists for the MD5 hash and collection.
// Parameters:
//   - ctx: context for cancellation and deadlines.
//   - md5Hash: MD5 hash of the meme content.
//   - collection: Qdrant collection name.
// Returns:
//   - bool: true if a record exists.
//   - error: non-nil if the lookup fails.
func (r *MemeVectorRepository) ExistsByMD5AndCollection(ctx context.Context, md5Hash, collection string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&domain.MemeVector{}).
		Where("md5_hash = ? AND collection = ?", md5Hash, collection).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetByMD5AndCollection retrieves a vector record by MD5 hash and collection.
// Parameters:
//   - ctx: context for cancellation and deadlines.
//   - md5Hash: MD5 hash of the meme content.
//   - collection: Qdrant collection name.
// Returns:
//   - *domain.MemeVector: matching vector record if found.
//   - error: non-nil if the lookup fails.
func (r *MemeVectorRepository) GetByMD5AndCollection(ctx context.Context, md5Hash, collection string) (*domain.MemeVector, error) {
	var vector domain.MemeVector
	if err := r.db.WithContext(ctx).
		Where("md5_hash = ? AND collection = ?", md5Hash, collection).
		First(&vector).Error; err != nil {
		return nil, err
	}
	return &vector, nil
}

// GetByMemeID retrieves all vector records for a given meme ID.
// Parameters:
//   - ctx: context for cancellation and deadlines.
//   - memeID: meme identifier.
// Returns:
//   - []domain.MemeVector: matching vector records.
//   - error: non-nil if the query fails.
func (r *MemeVectorRepository) GetByMemeID(ctx context.Context, memeID string) ([]domain.MemeVector, error) {
	var vectors []domain.MemeVector
	if err := r.db.WithContext(ctx).
		Where("meme_id = ?", memeID).
		Find(&vectors).Error; err != nil {
		return nil, err
	}
	return vectors, nil
}

// GetByCollection retrieves vector records for a given collection.
// Parameters:
//   - ctx: context for cancellation and deadlines.
//   - collection: Qdrant collection name.
//   - limit: maximum number of records to return.
//   - offset: number of records to skip.
// Returns:
//   - []domain.MemeVector: matching vector records.
//   - error: non-nil if the query fails.
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

// CountByCollection counts the number of vectors in a collection.
// Parameters:
//   - ctx: context for cancellation and deadlines.
//   - collection: Qdrant collection name.
// Returns:
//   - int64: number of vector records in the collection.
//   - error: non-nil if the query fails.
func (r *MemeVectorRepository) CountByCollection(ctx context.Context, collection string) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&domain.MemeVector{}).
		Where("collection = ?", collection).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// Delete removes a meme vector by ID.
// Parameters:
//   - ctx: context for cancellation and deadlines.
//   - id: vector record ID.
// Returns:
//   - error: non-nil if the delete fails.
func (r *MemeVectorRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&domain.MemeVector{}, "id = ?", id).Error
}

// DeleteByMemeIDAndCollection deletes a vector record by meme ID and collection.
// Parameters:
//   - ctx: context for cancellation and deadlines.
//   - memeID: meme identifier.
//   - collection: Qdrant collection name.
// Returns:
//   - error: non-nil if the delete fails.
func (r *MemeVectorRepository) DeleteByMemeIDAndCollection(ctx context.Context, memeID, collection string) error {
	return r.db.WithContext(ctx).
		Where("meme_id = ? AND collection = ?", memeID, collection).
		Delete(&domain.MemeVector{}).Error
}
