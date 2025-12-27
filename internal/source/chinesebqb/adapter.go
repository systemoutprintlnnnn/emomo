package chinesebqb

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
	SourceID   = "chinesebqb"
	SourceName = "ChineseBQB"
)

// Adapter implements the Source interface for ChineseBQB repository
type Adapter struct {
	repoPath string
	items    []source.MemeItem // Cached items
	loaded   bool
}

// NewAdapter creates a new ChineseBQB adapter
func NewAdapter(repoPath string) *Adapter {
	return &Adapter{
		repoPath: repoPath,
	}
}

// GetSourceID returns the unique identifier for this source
func (a *Adapter) GetSourceID() string {
	return SourceID
}

// GetDisplayName returns a human-readable name for this source
func (a *Adapter) GetDisplayName() string {
	return SourceName
}

// SupportsIncremental returns true if this source supports incremental updates
func (a *Adapter) SupportsIncremental() bool {
	return false // Static repository, no incremental updates
}

// FetchBatch fetches a batch of meme items
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

// loadItems scans the repository and loads all image items
func (a *Adapter) loadItems() error {
	// Check if repo path exists
	if _, err := os.Stat(a.repoPath); os.IsNotExist(err) {
		return fmt.Errorf("repository path does not exist: %s", a.repoPath)
	}

	a.items = []source.MemeItem{}

	// Walk through the repository
	err := filepath.Walk(a.repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Skip hidden files and non-image files
		name := info.Name()
		if strings.HasPrefix(name, ".") {
			return nil
		}

		// Check if it's an image file
		ext := strings.ToLower(filepath.Ext(name))
		format := ""
		switch ext {
		case ".jpg", ".jpeg":
			format = "jpeg"
		case ".png":
			format = "png"
		case ".gif":
			format = "gif"
		case ".webp":
			format = "webp"
		default:
			return nil // Skip non-image files
		}

		// Get category from parent directory name
		category := filepath.Base(filepath.Dir(path))
		if category == filepath.Base(a.repoPath) {
			category = "未分类"
		}

		// Generate source ID from path
		relPath, _ := filepath.Rel(a.repoPath, path)
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
		return fmt.Errorf("failed to walk repository: %w", err)
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
	// Remove extension
	name := strings.TrimSuffix(filename, filepath.Ext(filename))

	// Common patterns in ChineseBQB filenames
	// e.g., "熊猫头_无语.jpg" -> ["熊猫头", "无语"]
	parts := strings.Split(name, "_")
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

// GetTotalCount returns the total number of items
func (a *Adapter) GetTotalCount() (int, error) {
	if !a.loaded {
		if err := a.loadItems(); err != nil {
			return 0, err
		}
		a.loaded = true
	}
	return len(a.items), nil
}
