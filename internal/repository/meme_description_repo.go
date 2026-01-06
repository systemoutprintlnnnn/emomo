package repository

import (
	"context"

	"github.com/timmy/emomo/internal/domain"
	"gorm.io/gorm"
)

// MemeDescriptionRepository handles meme description data operations.
type MemeDescriptionRepository struct {
	db *gorm.DB
}

// NewMemeDescriptionRepository creates a new MemeDescriptionRepository.
// Parameters:
//   - db: GORM database handle used for queries.
//
// Returns:
//   - *MemeDescriptionRepository: repository instance bound to db.
func NewMemeDescriptionRepository(db *gorm.DB) *MemeDescriptionRepository {
	return &MemeDescriptionRepository{db: db}
}

// Create inserts a new meme description record.
// Parameters:
//   - ctx: context for cancellation and deadlines.
//   - desc: meme description record to persist.
//
// Returns:
//   - error: non-nil if the insert fails.
func (r *MemeDescriptionRepository) Create(ctx context.Context, desc *domain.MemeDescription) error {
	return r.db.WithContext(ctx).Create(desc).Error
}

// GetByMD5AndModel retrieves a description by MD5 hash and VLM model.
// Parameters:
//   - ctx: context for cancellation and deadlines.
//   - md5Hash: MD5 hash of the meme content.
//   - vlmModel: VLM model name used to generate the description.
//
// Returns:
//   - *domain.MemeDescription: matching description if found.
//   - error: non-nil if the lookup fails.
func (r *MemeDescriptionRepository) GetByMD5AndModel(ctx context.Context, md5Hash, vlmModel string) (*domain.MemeDescription, error) {
	var desc domain.MemeDescription
	if err := r.db.WithContext(ctx).
		Where("md5_hash = ? AND vlm_model = ?", md5Hash, vlmModel).
		First(&desc).Error; err != nil {
		return nil, err
	}
	return &desc, nil
}

// ExistsByMD5AndModel checks if a description exists for the MD5 hash and VLM model.
// Parameters:
//   - ctx: context for cancellation and deadlines.
//   - md5Hash: MD5 hash of the meme content.
//   - vlmModel: VLM model name used to generate the description.
//
// Returns:
//   - bool: true if a record exists.
//   - error: non-nil if the lookup fails.
func (r *MemeDescriptionRepository) ExistsByMD5AndModel(ctx context.Context, md5Hash, vlmModel string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&domain.MemeDescription{}).
		Where("md5_hash = ? AND vlm_model = ?", md5Hash, vlmModel).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetByID retrieves a description by its ID.
// Parameters:
//   - ctx: context for cancellation and deadlines.
//   - id: description record ID.
//
// Returns:
//   - *domain.MemeDescription: matching description if found.
//   - error: non-nil if the lookup fails.
func (r *MemeDescriptionRepository) GetByID(ctx context.Context, id string) (*domain.MemeDescription, error) {
	var desc domain.MemeDescription
	if err := r.db.WithContext(ctx).First(&desc, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &desc, nil
}

// GetByMemeID retrieves all descriptions for a given meme ID.
// Parameters:
//   - ctx: context for cancellation and deadlines.
//   - memeID: meme identifier.
//
// Returns:
//   - []domain.MemeDescription: matching description records.
//   - error: non-nil if the query fails.
func (r *MemeDescriptionRepository) GetByMemeID(ctx context.Context, memeID string) ([]domain.MemeDescription, error) {
	var descs []domain.MemeDescription
	if err := r.db.WithContext(ctx).
		Where("meme_id = ?", memeID).
		Find(&descs).Error; err != nil {
		return nil, err
	}
	return descs, nil
}

// Delete removes a meme description by ID.
// Parameters:
//   - ctx: context for cancellation and deadlines.
//   - id: description record ID.
//
// Returns:
//   - error: non-nil if the delete fails.
func (r *MemeDescriptionRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&domain.MemeDescription{}, "id = ?", id).Error
}
