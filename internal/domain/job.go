package domain

import "time"

// JobStatus represents the status of an ingest job.
// Values include JobStatusPending, JobStatusRunning, JobStatusCompleted, and JobStatusFailed.
type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
)

// IngestJob represents a data ingestion job and its progress metadata.
type IngestJob struct {
	ID             string     `gorm:"type:text;primaryKey" json:"id"`
	SourceID       string     `gorm:"type:text;not null;index" json:"source_id"`
	Status         JobStatus  `gorm:"default:pending" json:"status"`
	TotalItems     int        `gorm:"default:0" json:"total_items"`
	ProcessedItems int        `gorm:"default:0" json:"processed_items"`
	FailedItems    int        `gorm:"default:0" json:"failed_items"`
	StartedAt      *time.Time `json:"started_at,omitempty"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
	ErrorLog       string     `json:"error_log,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// TableName returns the database table name for IngestJob.
// Parameters: none.
// Returns:
//   - string: table name for GORM mapping.
func (IngestJob) TableName() string {
	return "ingest_jobs"
}
