package source

import "context"

// MemeItem represents a meme item from a data source.
type MemeItem struct {
	SourceID    string // Unique ID within the source
	URL         string // Image URL or local path
	Category    string // Category/folder name
	Tags        []string
	IsAnimated  bool
	Format      string // File format (jpg, png, gif, etc.)
	LocalPath   string // Local file path (if available)
}

// Source defines the interface for meme data sources.
type Source interface {
	// GetSourceID returns the unique identifier for this source.
	// Parameters: none.
	// Returns:
	//   - string: stable source identifier.
	GetSourceID() string

	// GetDisplayName returns a human-readable name for this source.
	// Parameters: none.
	// Returns:
	//   - string: display-friendly source name.
	GetDisplayName() string

	// FetchBatch fetches a batch of meme items starting from the given cursor.
	// Parameters:
	//   - ctx: context for cancellation and deadlines.
	//   - cursor: pagination cursor or empty for first page.
	//   - limit: maximum number of items to fetch.
	// Returns:
	//   - items: batch of meme items.
	//   - nextCursor: cursor for the next batch or empty if done.
	//   - err: non-nil if fetching fails.
	FetchBatch(ctx context.Context, cursor string, limit int) (items []MemeItem, nextCursor string, err error)

	// SupportsIncremental returns true if this source supports incremental updates.
	// Parameters: none.
	// Returns:
	//   - bool: true when incremental updates are supported.
	SupportsIncremental() bool
}
