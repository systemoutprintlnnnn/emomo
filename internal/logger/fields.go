package logger

// Fields is an alias for map[string]interface{} for convenience.
type Fields map[string]interface{}

// ============================================
// Standard Tracing Fields (Context level)
// These fields are propagated through the call chain
// ============================================

const (
	// FieldRequestID is the HTTP request ID (UUID)
	FieldRequestID = "request_id"

	// FieldJobID is the data ingestion job ID
	FieldJobID = "job_id"

	// FieldSearchID is the search request ID
	FieldSearchID = "search_id"

	// FieldComponent is the component/module name
	FieldComponent = "component"

	// FieldSource is the data source identifier
	FieldSource = "source"

	// FieldUserID is the user ID (reserved for future use)
	FieldUserID = "user_id"
)

// ============================================
// Standard Metric Fields (Entry level)
// These fields are used for aggregation and alerting
// ============================================

const (
	// FieldDurationMs is the execution duration in milliseconds
	FieldDurationMs = "duration_ms"

	// FieldCount is a generic count field
	FieldCount = "count"

	// FieldSize is the data size in bytes
	FieldSize = "size"

	// FieldStatus is the operation status
	FieldStatus = "status"
)
