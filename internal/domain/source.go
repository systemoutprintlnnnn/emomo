package domain

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// SourceType represents the type of data source
type SourceType string

const (
	SourceTypeStatic  SourceType = "static"
	SourceTypeAPI     SourceType = "api"
	SourceTypeCrawler SourceType = "crawler"
)

// SourceConfig is a custom type for storing JSON config in SQLite
type SourceConfig map[string]interface{}

func (c SourceConfig) Value() (driver.Value, error) {
	if c == nil {
		return "{}", nil
	}
	return json.Marshal(c)
}

func (c *SourceConfig) Scan(value interface{}) error {
	if value == nil {
		*c = SourceConfig{}
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		str, ok := value.(string)
		if !ok {
			return errors.New("failed to scan SourceConfig")
		}
		bytes = []byte(str)
	}
	return json.Unmarshal(bytes, c)
}

// DataSource represents a meme data source configuration
type DataSource struct {
	ID             string       `gorm:"type:text;primaryKey" json:"id"`
	Name           string       `gorm:"type:text;not null" json:"name"`
	Type           SourceType   `gorm:"type:text;not null" json:"type"`
	Config         SourceConfig `gorm:"type:text" json:"config"`
	LastSyncAt     *time.Time   `json:"last_sync_at,omitempty"`
	LastSyncCursor string       `gorm:"type:text" json:"last_sync_cursor,omitempty"`
	IsEnabled      bool         `gorm:"default:true" json:"is_enabled"`
	Priority       int          `gorm:"default:0" json:"priority"`
	CreatedAt      time.Time    `json:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at"`
}

func (DataSource) TableName() string {
	return "data_sources"
}
