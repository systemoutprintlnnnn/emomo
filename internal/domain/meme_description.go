package domain

import "time"

// MemeDescription represents a VLM-generated description for a meme.
// This allows the same meme to have multiple descriptions from different VLM models.
type MemeDescription struct {
	ID          string    `gorm:"type:text;primaryKey" json:"id"`
	MemeID      string    `gorm:"type:text;not null;index:idx_meme_descriptions_meme" json:"meme_id"`
	MD5Hash     string    `gorm:"type:text;not null;uniqueIndex:idx_meme_descriptions_md5_model" json:"md5_hash"`
	VLMModel    string    `gorm:"type:text;not null;uniqueIndex:idx_meme_descriptions_md5_model" json:"vlm_model"`
	Description string    `gorm:"type:text;not null" json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

// TableName returns the database table name for MemeDescription.
func (MemeDescription) TableName() string {
	return "meme_descriptions"
}
