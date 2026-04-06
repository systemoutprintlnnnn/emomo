package service

import (
	"reflect"
	"testing"
)

func TestSearchServiceGetAvailableCollectionsUsesConfiguredKeys(t *testing.T) {
	t.Parallel()

	searchService := NewSearchService(nil, nil, nil, nil, nil, nil, nil, &SearchConfig{
		DefaultCollection: "qwen3",
	})

	searchService.RegisterCollection("jina", nil, nil)
	searchService.RegisterCollection("qwen3", nil, nil)
	searchService.RegisterCollection("alpha", nil, nil)

	got := searchService.GetAvailableCollections()
	want := []string{"qwen3", "alpha", "jina"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetAvailableCollections() = %v, want %v", got, want)
	}
}
