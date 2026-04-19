package staging

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/timmy/emomo/internal/source"
)

const (
	// ManifestFileName is the JSONL manifest file name in staging sources.
	ManifestFileName = "manifest.jsonl"
	// ImagesDir is the directory name for staged images.
	ImagesDir = "images"
)

// ManifestItem represents an item in the manifest.jsonl file.
type ManifestItem struct {
	ID         string   `json:"id"`
	Filename   string   `json:"filename"`
	Category   string   `json:"category"`
	Tags       []string `json:"tags"`
	SourceURL  string   `json:"source_url"`
	IsAnimated bool     `json:"is_animated"`
	Format     string   `json:"format"`
	CrawledAt  string   `json:"crawled_at"`
}

// Adapter implements the Source interface for the staging directory.
type Adapter struct {
	basePath string
	sourceID string
	items    []source.MemeItem
	loaded   bool
}

// NewAdapter creates a new staging adapter.
// Parameters:
//   - basePath: base path to the staging directory.
//   - sourceID: identifier for the staging source.
// Returns:
//   - *Adapter: initialized staging adapter.
func NewAdapter(basePath, sourceID string) *Adapter {
	return &Adapter{
		basePath: basePath,
		sourceID: sourceID,
	}
}

// GetSourceID returns the unique identifier for this source.
// Parameters: none.
// Returns:
//   - string: source identifier with "staging:" prefix.
func (a *Adapter) GetSourceID() string {
	return "staging:" + a.sourceID
}

// GetDisplayName returns a human-readable name for this source.
// Parameters: none.
// Returns:
//   - string: display name for the staging source.
func (a *Adapter) GetDisplayName() string {
	return fmt.Sprintf("Staging (%s)", a.sourceID)
}

// SupportsIncremental returns true if this source supports incremental updates.
// Parameters: none.
// Returns:
//   - bool: false for staging sources.
func (a *Adapter) SupportsIncremental() bool {
	return false
}

// FetchBatch fetches a batch of meme items from the staging directory.
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
			return nil, "", fmt.Errorf("failed to load staging items: %w", err)
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
		return []source.MemeItem{}, "", nil
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

// loadItems loads all items from the manifest file
func (a *Adapter) loadItems() error {
	stagingPath := filepath.Join(a.basePath, a.sourceID)
	manifestPath := filepath.Join(stagingPath, ManifestFileName)
	imagesPath := filepath.Join(stagingPath, ImagesDir)

	// Check if manifest exists
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		return fmt.Errorf("manifest file not found: %s. Run the Python crawler first to populate staging", manifestPath)
	}

	// Open manifest file
	file, err := os.Open(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to open manifest: %w", err)
	}
	defer file.Close()

	a.items = []source.MemeItem{}

	// Read line by line (JSON Lines format)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var item ManifestItem
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			// Skip malformed lines
			continue
		}

		// Build local path
		localPath := filepath.Join(imagesPath, item.Filename)

		// Verify file exists
		if _, err := os.Stat(localPath); os.IsNotExist(err) {
			// Skip if image file doesn't exist
			continue
		}

		// Convert to MemeItem
		memeItem := source.MemeItem{
			SourceID:   fmt.Sprintf("%s_%s", a.sourceID, item.ID),
			URL:        item.SourceURL,
			LocalPath:  localPath,
			Category:   item.Category,
			Tags:       item.Tags,
			IsAnimated: item.IsAnimated,
			Format:     item.Format,
		}

		a.items = append(a.items, memeItem)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading manifest: %w", err)
	}

	// Sort items by ID for consistent ordering
	sort.Slice(a.items, func(i, j int) bool {
		return a.items[i].SourceID < a.items[j].SourceID
	})

	return nil
}

// GetTotalCount returns the total number of items in staging.
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

// ListStagingSources lists all available staging sources.
// Parameters:
//   - basePath: base path to the staging directory.
// Returns:
//   - []string: list of staging source IDs.
//   - error: non-nil if reading the directory fails.
func ListStagingSources(basePath string) ([]string, error) {
	entries, err := os.ReadDir(basePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	var sources []string
	for _, entry := range entries {
		if entry.IsDir() {
			manifestPath := filepath.Join(basePath, entry.Name(), ManifestFileName)
			if _, err := os.Stat(manifestPath); err == nil {
				sources = append(sources, entry.Name())
			}
		}
	}

	return sources, nil
}
