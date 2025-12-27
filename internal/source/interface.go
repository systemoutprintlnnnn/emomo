package source

import "context"

// MemeItem represents a meme item from a data source
type MemeItem struct {
	SourceID    string // Unique ID within the source
	URL         string // Image URL or local path
	Category    string // Category/folder name
	Tags        []string
	IsAnimated  bool
	Format      string // File format (jpg, png, gif, etc.)
	LocalPath   string // Local file path (if available)
}

// Source defines the interface for meme data sources
type Source interface {
	// GetSourceID returns the unique identifier for this source
	GetSourceID() string

	// GetDisplayName returns a human-readable name for this source
	GetDisplayName() string

	// FetchBatch fetches a batch of meme items starting from the given cursor
	// Returns the items, the next cursor (empty if no more items), and any error
	FetchBatch(ctx context.Context, cursor string, limit int) (items []MemeItem, nextCursor string, err error)

	// SupportsIncremental returns true if this source supports incremental updates
	SupportsIncremental() bool
}
