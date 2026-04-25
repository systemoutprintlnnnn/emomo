package main

import (
	"testing"

	"github.com/timmy/emomo/internal/config"
)

func TestBuildSourcesRespectsLocalDirEnabled(t *testing.T) {
	cfg := &config.Config{}
	cfg.Sources.LocalDir.Enabled = false
	cfg.Sources.LocalDir.RootPath = "/tmp/memes"

	sources := buildSources(cfg)

	if len(sources) != 0 {
		t.Fatalf("expected no sources when localdir is disabled, got %d", len(sources))
	}
}

func TestBuildSourcesRegistersLocalDirWhenEnabled(t *testing.T) {
	cfg := &config.Config{}
	cfg.Sources.LocalDir.Enabled = true
	cfg.Sources.LocalDir.RootPath = "/tmp/memes"

	sources := buildSources(cfg)

	src, ok := sources["localdir"]
	if !ok {
		t.Fatal("expected localdir source to be registered")
	}
	if got := src.GetSourceID(); got != "localdir" {
		t.Fatalf("expected source id localdir, got %q", got)
	}
}
