package domain

import "time"

// MemeVector represents the relationship between a meme and its vector in a specific collection.
// This allows the same meme to be embedded using different models and stored per collection.
type MemeVector struct {
	ID                string    `gorm:"type:text;primaryKey" json:"id"`
	MemeID            string    `gorm:"type:text;not null;index:idx_meme_vectors_meme" json:"meme_id"`
	MD5Hash           string    `gorm:"type:text;not null;uniqueIndex:idx_meme_vectors_md5_collection_type" json:"md5_hash"`
	Collection        string    `gorm:"type:text;not null;uniqueIndex:idx_meme_vectors_md5_collection_type" json:"collection"`
	VectorType        string    `gorm:"type:text;not null;default:image;uniqueIndex:idx_meme_vectors_md5_collection_type" json:"vector_type"`
	EmbeddingModel    string    `gorm:"type:text;not null" json:"embedding_model"`
	EmbeddingProvider string    `gorm:"type:text" json:"embedding_provider"`
	EmbeddingMode     string    `gorm:"type:text;not null;default:independent" json:"embedding_mode"`
	Dimension         int       `json:"dimension"`
	InputHash         string    `gorm:"type:text" json:"input_hash"`
	DescriptionID     string    `gorm:"type:text;index:idx_meme_vectors_description" json:"description_id"`
	QdrantPointID     string    `gorm:"type:text;not null" json:"qdrant_point_id"`
	Status            string    `gorm:"type:text;default:active" json:"status"`
	CreatedAt         time.Time `json:"created_at"`
}

// TableName returns the database table name for MemeVector.
// Parameters: none.
// Returns:
//   - string: table name for GORM mapping.
func (MemeVector) TableName() string {
	return "meme_vectors"
}

// MemeVectorStatus constants for tracking vector lifecycle.
const (
	MemeVectorStatusActive  = "active"
	MemeVectorStatusDeleted = "deleted"
)

// MemeVectorType constants describe what input produced a vector.
const (
	MemeVectorTypeImage   = "image"
	MemeVectorTypeCaption = "caption"
	MemeVectorTypeFused   = "fused"
)

// MemeVectorEmbeddingMode constants describe how a vector was generated.
const (
	MemeVectorEmbeddingModeIndependent = "independent"
	MemeVectorEmbeddingModeFusion      = "fusion"
)
