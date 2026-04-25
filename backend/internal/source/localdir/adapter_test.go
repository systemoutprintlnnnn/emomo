package localdir

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func writeFile(t *testing.T, path string, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func TestFetchBatchScansStaticImagesAndSkipsUnsupportedFiles(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "cat", "hello.jpg"), "jpg")
	writeFile(t, filepath.Join(root, "cat", "wave.png"), "png")
	writeFile(t, filepath.Join(root, "root.webp"), "webp")
	writeFile(t, filepath.Join(root, "cat", "animated.gif"), "gif")
	writeFile(t, filepath.Join(root, ".DS_Store"), "ignored")

	adapter := NewAdapter(Options{RootPath: root})
	items, nextCursor, err := adapter.FetchBatch(context.Background(), "", 10)
	if err != nil {
		t.Fatalf("FetchBatch() error = %v", err)
	}
	if nextCursor != "" {
		t.Fatalf("FetchBatch() nextCursor = %q, want empty", nextCursor)
	}
	if len(items) != 3 {
		t.Fatalf("FetchBatch() returned %d items, want 3", len(items))
	}

	byID := map[string]string{}
	for _, item := range items {
		byID[item.SourceID] = item.Category
		if item.Format == "gif" {
			t.Fatalf("FetchBatch() returned GIF item: %+v", item)
		}
	}

	if got := byID["cat/hello.jpg"]; got != "cat" {
		t.Fatalf("category for cat/hello.jpg = %q, want cat", got)
	}
	if got := byID["root.webp"]; got != "未分类" {
		t.Fatalf("category for root.webp = %q, want 未分类", got)
	}
	for _, item := range items {
		if item.SourceID == "cat/hello.jpg" {
			for _, tag := range []string{"cat", "hello"} {
				if !slices.Contains(item.Tags, tag) {
					t.Fatalf("Tags = %v, want tag %q", item.Tags, tag)
				}
			}
		}
	}
}

func TestFetchBatchUsesXiaohongshuManifestAndQueueMetadata(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	keepImage := filepath.Join(root, "65d4a17900000000070079da_1.jpg")
	rejectImage := filepath.Join(root, "65d4a17900000000070079da_2.jpg")
	unlistedImage := filepath.Join(root, "unlisted_1.jpg")
	writeFile(t, keepImage, "webp bytes")
	writeFile(t, rejectImage, "webp bytes")
	writeFile(t, unlistedImage, "webp bytes")

	manifestPath := filepath.Join(root, "stage2_results.jsonl")
	writeFile(t, manifestPath,
		`{"note_id":"65d4a17900000000070079da","filename":"65d4a17900000000070079da_1.jpg","keyword":"学生党表情包","title":"考试周","confidence":0.9,"reason":"熊猫头配文字","keep":true}`+"\n"+
			`{"note_id":"65d4a17900000000070079da","filename":"65d4a17900000000070079da_2.jpg","keyword":"学生党表情包","confidence":0.4,"reason":"非表情包","keep":false}`+"\n")

	queuePath := filepath.Join(root, "stage1_queue.jsonl")
	writeFile(t, queuePath,
		`{"note_id":"65d4a17900000000070079da","filename":"65d4a17900000000070079da_1.jpg","keyword":"学生党表情包","keywords":["学生党表情包","考试表情包"],"title":"考试周","author":"alice","published_at":"2026-01-01"}`+"\n")

	adapter := NewAdapter(Options{
		RootPath:     root,
		SourceID:     "xiaohongshu",
		ManifestPath: manifestPath,
		QueuePath:    queuePath,
	})

	items, nextCursor, err := adapter.FetchBatch(context.Background(), "", 10)
	if err != nil {
		t.Fatalf("FetchBatch() error = %v", err)
	}
	if nextCursor != "" {
		t.Fatalf("FetchBatch() nextCursor = %q, want empty", nextCursor)
	}
	if len(items) != 1 {
		t.Fatalf("FetchBatch() returned %d items, want 1", len(items))
	}

	item := items[0]
	if got := adapter.GetSourceID(); got != "xiaohongshu" {
		t.Fatalf("GetSourceID() = %q, want xiaohongshu", got)
	}
	if item.SourceID != "65d4a17900000000070079da:65d4a17900000000070079da_1.jpg" {
		t.Fatalf("SourceID = %q", item.SourceID)
	}
	if item.LocalPath != keepImage {
		t.Fatalf("LocalPath = %q, want %q", item.LocalPath, keepImage)
	}
	if item.Category != "学生党表情包" {
		t.Fatalf("Category = %q, want 学生党表情包", item.Category)
	}
	for _, tag := range []string{"学生党表情包", "考试表情包", "小红书"} {
		if !slices.Contains(item.Tags, tag) {
			t.Fatalf("Tags = %v, want tag %q", item.Tags, tag)
		}
	}
}
