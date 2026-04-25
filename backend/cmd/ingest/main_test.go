package main

import (
	"strings"
	"testing"

	"github.com/timmy/emomo/internal/config"
)

func TestSelectSourceRejectsStagingSources(t *testing.T) {
	cfg := &config.Config{}
	cfg.Sources.ChineseBQB.Enabled = true
	cfg.Sources.ChineseBQB.RepoPath = "/tmp/chinesebqb"

	_, err := selectSource(cfg, "staging:legacy")

	if err == nil {
		t.Fatal("expected staging source to be rejected")
	}
	if !strings.Contains(err.Error(), "unsupported source type") {
		t.Fatalf("expected unsupported source type error, got %v", err)
	}
}

func TestSelectSourceReturnsChineseBQBWhenEnabled(t *testing.T) {
	cfg := &config.Config{}
	cfg.Sources.ChineseBQB.Enabled = true
	cfg.Sources.ChineseBQB.RepoPath = "/tmp/chinesebqb"

	src, err := selectSource(cfg, "chinesebqb")

	if err != nil {
		t.Fatalf("expected chinesebqb source, got error %v", err)
	}
	if got := src.GetSourceID(); got != "chinesebqb" {
		t.Fatalf("expected source id chinesebqb, got %q", got)
	}
}

func TestSelectSourceRejectsDisabledChineseBQB(t *testing.T) {
	cfg := &config.Config{}
	cfg.Sources.ChineseBQB.Enabled = false
	cfg.Sources.ChineseBQB.RepoPath = "/tmp/chinesebqb"

	_, err := selectSource(cfg, "chinesebqb")

	if err == nil {
		t.Fatal("expected disabled chinesebqb source to be rejected")
	}
	if !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("expected disabled source error, got %v", err)
	}
}
