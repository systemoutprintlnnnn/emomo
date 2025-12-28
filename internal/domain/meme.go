package domain

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// MemeStatus represents the processing status of a meme
type MemeStatus string

const (
	MemeStatusPending MemeStatus = "pending"
	MemeStatusActive  MemeStatus = "active"
	MemeStatusFailed  MemeStatus = "failed"
)

// StringArray is a custom type for storing string arrays as JSON in SQLite
type StringArray []string

func (a StringArray) Value() (driver.Value, error) {
	if a == nil {
		return "[]", nil
	}
	b, err := json.Marshal(a)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func (a *StringArray) Scan(value interface{}) error {
	if value == nil {
		*a = StringArray{}
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		str, ok := value.(string)
		if !ok {
			return errors.New("failed to scan StringArray")
		}
		bytes = []byte(str)
	}
	return json.Unmarshal(bytes, a)
}

// Meme represents a meme/sticker in the system
type Meme struct {
	ID             string      `gorm:"type:text;primaryKey" json:"id"`
	SourceType     string      `gorm:"type:text;not null;index:idx_memes_source,unique" json:"source_type"`
	SourceID       string      `gorm:"type:text;not null;index:idx_memes_source,unique" json:"source_id"`
	StorageKey     string      `gorm:"type:text" json:"storage_key"`
	LocalPath      string      `gorm:"column:local_path" json:"local_path,omitempty"`
	Width          int         `json:"width"`
	Height         int         `json:"height"`
	Format         string      `json:"format"`
	IsAnimated     bool        `json:"is_animated"`
	FileSize       int64       `json:"file_size"`
	MD5Hash        string      `gorm:"index:idx_memes_md5" json:"md5_hash"`
	PerceptualHash string      `gorm:"type:text" json:"perceptual_hash,omitempty"`
	QdrantPointID  string      `gorm:"type:text" json:"qdrant_point_id,omitempty"`
	VLMDescription string      `gorm:"type:text" json:"vlm_description,omitempty"`
	VLMModel       string      `gorm:"type:text" json:"vlm_model,omitempty"`
	EmbeddingModel string      `gorm:"type:text" json:"embedding_model,omitempty"`
	Tags           StringArray `gorm:"type:text" json:"tags"`
	Category       string      `gorm:"type:text;index:idx_memes_category" json:"category"`
	Status         MemeStatus  `gorm:"type:text;index:idx_memes_status;default:pending" json:"status"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
}

func (Meme) TableName() string {
	return "memes"
}

// MemeSearchResult represents a search result with similarity score
type MemeSearchResult struct {
	Meme
	Score float32 `json:"score"`
}
