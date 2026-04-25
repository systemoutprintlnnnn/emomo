package main

import (
	"testing"

	"github.com/timmy/emomo/internal/config"
)

func TestBuildSourcesRespectsChineseBQBEnabled(t *testing.T) {
	cfg := &config.Config{}
	cfg.Sources.ChineseBQB.Enabled = false
	cfg.Sources.ChineseBQB.RepoPath = "/tmp/chinesebqb"

	sources := buildSources(cfg)

	if len(sources) != 0 {
		t.Fatalf("expected no sources when ChineseBQB is disabled, got %d", len(sources))
	}
}

func TestBuildSourcesRegistersChineseBQBWhenEnabled(t *testing.T) {
	cfg := &config.Config{}
	cfg.Sources.ChineseBQB.Enabled = true
	cfg.Sources.ChineseBQB.RepoPath = "/tmp/chinesebqb"

	sources := buildSources(cfg)

	src, ok := sources["chinesebqb"]
	if !ok {
		t.Fatal("expected chinesebqb source to be registered")
	}
	if got := src.GetSourceID(); got != "chinesebqb" {
		t.Fatalf("expected source id chinesebqb, got %q", got)
	}
}
