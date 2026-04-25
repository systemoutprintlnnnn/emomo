package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/timmy/emomo/internal/source"
)

func TestIsSupportedStaticImageFormatRejectsGIF(t *testing.T) {
	t.Parallel()

	if isSupportedStaticImageFormat("gif") {
		t.Fatal("isSupportedStaticImageFormat(\"gif\") = true, want false")
	}
}

func TestProcessItemRejectsGIFMagicBytes(t *testing.T) {
	t.Parallel()

	imagePath := filepath.Join(t.TempDir(), "looks-static.jpg")
	if err := os.WriteFile(imagePath, []byte("GIF89a-static"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	service := &IngestService{}
	err := service.processItem(context.Background(), "test", &source.MemeItem{
		SourceID:  "deceptive-gif",
		LocalPath: imagePath,
		Format:    "jpeg",
	}, &IngestOptions{})

	if !errors.Is(err, errSkipUnsupportedImageFormat) {
		t.Fatalf("processItem() error = %v, want errSkipUnsupportedImageFormat", err)
	}
}
