package domain

import "time"

// MemeVector represents the relationship between a meme and its vector in a specific collection.
// This allows the same meme to be embedded using different embedding models,
// with each vector stored in its own Qdrant collection.
type MemeVector struct {
	ID             string    `gorm:"type:text;primaryKey" json:"id"`
	MemeID         string    `gorm:"type:text;not null;index:idx_meme_vectors_meme" json:"meme_id"`
	MD5Hash        string    `gorm:"type:text;not null;uniqueIndex:idx_meme_vectors_md5_collection" json:"md5_hash"`
	Collection     string    `gorm:"type:text;not null;uniqueIndex:idx_meme_vectors_md5_collection" json:"collection"`
	EmbeddingModel string    `gorm:"type:text;not null" json:"embedding_model"`
	QdrantPointID  string    `gorm:"type:text;not null" json:"qdrant_point_id"`
	Status         string    `gorm:"type:text;default:active" json:"status"`
	CreatedAt      time.Time `json:"created_at"`
}

func (MemeVector) TableName() string {
	return "meme_vectors"
}

// MemeVectorStatus constants
const (
	MemeVectorStatusActive  = "active"
	MemeVectorStatusDeleted = "deleted"
)
