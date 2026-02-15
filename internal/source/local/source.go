package local

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/timmy/emomo/internal/source"
)

const (
	// SourceID is the identifier for the local folder source.
	SourceID = "local"
	// SourceName is the display name for the local folder source.
	SourceName = "Local Folder"
)

// Adapter implements the Source interface for a local folder.
type Adapter struct {
	folderPath string
	items      []source.MemeItem // Cached items
	loaded     bool
}

// NewAdapter creates a new local folder adapter.
// Parameters:
//   - folderPath: filesystem path to the folder containing memes.
// Returns:
//   - *Adapter: initialized adapter instance.
func NewAdapter(folderPath string) *Adapter {
	return &Adapter{
		folderPath: folderPath,
	}
}

// GetSourceID returns the unique identifier for this source.
// Parameters: none.
// Returns:
//   - string: source identifier string.
func (a *Adapter) GetSourceID() string {
	return SourceID
}

// GetDisplayName returns a human-readable name for this source.
// Parameters: none.
// Returns:
//   - string: display name for the source.
func (a *Adapter) GetDisplayName() string {
	return SourceName
}

// SupportsIncremental returns true if this source supports incremental updates.
// Parameters: none.
// Returns:
//   - bool: false for the local folder.
func (a *Adapter) SupportsIncremental() bool {
	return false
}

// FetchBatch fetches a batch of meme items.
// Parameters:
//   - ctx: context for cancellation and deadlines (unused for local reads).
//   - cursor: pagination cursor as an index string.
//   - limit: maximum number of items to fetch.
// Returns:
//   - []source.MemeItem: batch of meme items.
//   - string: next cursor or empty if no more items.
//   - error: non-nil if loading or parsing fails.
func (a *Adapter) FetchBatch(ctx context.Context, cursor string, limit int) ([]source.MemeItem, string, error) {
	// Load all items on first call
	if !a.loaded {
		if err := a.loadItems(); err != nil {
			return nil, "", fmt.Errorf("failed to load items: %w", err)
		}
		a.loaded = true
	}

	// Parse cursor (index)
	startIndex := 0
	if cursor != "" {
		var err error
		startIndex, err = strconv.Atoi(cursor)
		if err != nil {
			return nil, "", fmt.Errorf("invalid cursor: %w", err)
		}
	}

	// Check bounds
	if startIndex >= len(a.items) {
		return []source.MemeItem{}, "", nil // No more items
	}

	// Calculate end index
	endIndex := startIndex + limit
	if endIndex > len(a.items) {
		endIndex = len(a.items)
	}

	// Get batch
	batch := a.items[startIndex:endIndex]

	// Calculate next cursor
	nextCursor := ""
	if endIndex < len(a.items) {
		nextCursor = strconv.Itoa(endIndex)
	}

	return batch, nextCursor, nil
}

// loadItems scans the folder and loads all image items
func (a *Adapter) loadItems() error {
	// Check if folder path exists
	if _, err := os.Stat(a.folderPath); os.IsNotExist(err) {
		return fmt.Errorf("local folder path does not exist: %s", a.folderPath)
	}

	a.items = []source.MemeItem{}

	// Supported image formats
	supportedFormats := map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		".gif":  true,
		".webp": true,
		".bmp":  true,
	}

	// Walk through the folder
	err := filepath.Walk(a.folderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Skip hidden files
		name := info.Name()
		if strings.HasPrefix(name, ".") {
			return nil
		}

		// Check if it's an image file
		ext := strings.ToLower(filepath.Ext(name))
		if !supportedFormats[ext] {
			return nil // Skip non-image files
		}

		// Determine format
		format := strings.TrimPrefix(ext, ".")

		// Get category from parent directory name
		category := filepath.Base(filepath.Dir(path))
		if category == filepath.Base(a.folderPath) {
			category = "未分类"
		}

		// Generate source ID from path
		relPath, _ := filepath.Rel(a.folderPath, path)
		sourceID := strings.ReplaceAll(relPath, string(os.PathSeparator), "_")

		item := source.MemeItem{
			SourceID:   sourceID,
			URL:        path, // Local file path
			LocalPath:  path,
			Category:   category,
			Format:     format,
			IsAnimated: format == "gif",
			Tags:       extractTags(category, name),
		}

		a.items = append(a.items, item)
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk folder: %w", err)
	}

	// Sort items by source ID for consistent ordering
	sort.Slice(a.items, func(i, j int) bool {
		return a.items[i].SourceID < a.items[j].SourceID
	})

	return nil
}

// extractTags extracts tags from category and filename
func extractTags(category, filename string) []string {
	tags := []string{}

	// Add category as a tag
	if category != "" && category != "未分类" {
		tags = append(tags, category)
	}

	// Extract potential tags from filename
	name := strings.TrimSuffix(filename, filepath.Ext(filename))

	// Split by common separators
	parts := strings.FieldsFunc(name, func(r rune) bool {
		return r == '_' || r == '-' || r == ' ' || r == '.'
	})

	for _, part := range parts {
		part = strings.TrimSpace(part)
		// Skip numeric-only parts and very short parts
		if len(part) > 1 && !isNumeric(part) {
			tags = append(tags, part)
		}
	}

	return uniqueStrings(tags)
}

func isNumeric(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func uniqueStrings(strs []string) []string {
	seen := make(map[string]bool)
	result := []string{}
	for _, s := range strs {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

// GetTotalCount returns the total number of items.
// Parameters: none.
// Returns:
//   - int: total item count.
//   - error: non-nil if loading fails.
func (a *Adapter) GetTotalCount() (int, error) {
	if !a.loaded {
		if err := a.loadItems(); err != nil {
			return 0, err
		}
		a.loaded = true
	}
	return len(a.items), nil
}
