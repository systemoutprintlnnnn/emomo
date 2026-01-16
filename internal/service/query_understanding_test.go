package service

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestQueryUnderstandingService_Disabled(t *testing.T) {
	svc := NewQueryUnderstandingService(nil)

	if svc.IsEnabled() {
		t.Error("expected service to be disabled")
	}

	plan, err := svc.Understand(context.Background(), "test query")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if plan == nil {
		t.Fatal("expected plan to be non-nil")
	}

	// Should return fallback
	if plan.SemanticQuery != "test query" {
		t.Errorf("expected semantic query to be original, got %q", plan.SemanticQuery)
	}
}

func TestQueryUnderstandingService_FallbackUnderstand(t *testing.T) {
	svc := &QueryUnderstandingService{enabled: false}

	tests := []struct {
		name           string
		query          string
		expectedIntent QueryIntent
		minDenseWeight float32
		maxDenseWeight float32
	}{
		{
			name:           "emotion query",
			query:          "无语的表情",
			expectedIntent: IntentEmotion,
			minDenseWeight: 0.7,
			maxDenseWeight: 0.9,
		},
		{
			name:           "subject query - panda",
			query:          "熊猫头",
			expectedIntent: IntentSubject,
			minDenseWeight: 0.4,
			maxDenseWeight: 0.6,
		},
		{
			name:           "composite query",
			query:          "熊猫头无语",
			expectedIntent: IntentComposite,
			minDenseWeight: 0.5,
			maxDenseWeight: 0.7,
		},
		{
			name:           "meme query",
			query:          "笑死我了表情包",
			expectedIntent: IntentMeme,
			minDenseWeight: 0.5,
			maxDenseWeight: 0.7,
		},
		{
			name:           "text query with digits",
			query:          "有666的",
			expectedIntent: IntentText,
			minDenseWeight: 0.2,
			maxDenseWeight: 0.4,
		},
		{
			name:           "text query with quotes",
			query:          `"谢谢"`,
			expectedIntent: IntentText,
			minDenseWeight: 0.2,
			maxDenseWeight: 0.4,
		},
		{
			name:           "short semantic query",
			query:          "开心",
			expectedIntent: IntentEmotion,
			minDenseWeight: 0.7,
			maxDenseWeight: 0.9,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := svc.fallbackUnderstand(tt.query)

			if plan.Intent != tt.expectedIntent {
				t.Errorf("expected intent %s, got %s", tt.expectedIntent, plan.Intent)
			}

			if plan.Strategy.DenseWeight < tt.minDenseWeight || plan.Strategy.DenseWeight > tt.maxDenseWeight {
				t.Errorf("expected dense weight between %.1f and %.1f, got %.1f",
					tt.minDenseWeight, tt.maxDenseWeight, plan.Strategy.DenseWeight)
			}

			if len(plan.Keywords) == 0 {
				t.Error("expected at least one keyword")
			}
		})
	}
}

func TestQueryUnderstandingService_ValidateAndFix(t *testing.T) {
	svc := &QueryUnderstandingService{enabled: true}

	tests := []struct {
		name     string
		plan     *QueryPlan
		query    string
		validate func(*testing.T, *QueryPlan)
	}{
		{
			name: "fix negative dense weight",
			plan: &QueryPlan{
				Intent: IntentEmotion,
				Strategy: SearchStrategy{
					DenseWeight: -0.5,
				},
			},
			query: "test",
			validate: func(t *testing.T, p *QueryPlan) {
				if p.Strategy.DenseWeight != 0 {
					t.Errorf("expected dense weight 0, got %f", p.Strategy.DenseWeight)
				}
			},
		},
		{
			name: "fix dense weight over 1",
			plan: &QueryPlan{
				Intent: IntentEmotion,
				Strategy: SearchStrategy{
					DenseWeight: 1.5,
				},
			},
			query: "test",
			validate: func(t *testing.T, p *QueryPlan) {
				if p.Strategy.DenseWeight != 1 {
					t.Errorf("expected dense weight 1, got %f", p.Strategy.DenseWeight)
				}
			},
		},
		{
			name: "fix empty semantic query",
			plan: &QueryPlan{
				Intent:        IntentEmotion,
				SemanticQuery: "",
			},
			query: "original query",
			validate: func(t *testing.T, p *QueryPlan) {
				if p.SemanticQuery != "original query" {
					t.Errorf("expected semantic query to be original, got %q", p.SemanticQuery)
				}
			},
		},
		{
			name: "fix empty keywords",
			plan: &QueryPlan{
				Intent:   IntentEmotion,
				Keywords: []string{},
			},
			query: "test query",
			validate: func(t *testing.T, p *QueryPlan) {
				if len(p.Keywords) != 1 || p.Keywords[0] != "test query" {
					t.Errorf("expected keywords to contain original query, got %v", p.Keywords)
				}
			},
		},
		{
			name: "truncate too many keywords",
			plan: &QueryPlan{
				Intent:   IntentEmotion,
				Keywords: []string{"a", "b", "c", "d", "e", "f", "g"},
			},
			query: "test",
			validate: func(t *testing.T, p *QueryPlan) {
				if len(p.Keywords) != 5 {
					t.Errorf("expected 5 keywords, got %d", len(p.Keywords))
				}
			},
		},
		{
			name: "truncate too many synonyms",
			plan: &QueryPlan{
				Intent:   IntentEmotion,
				Synonyms: []string{"a", "b", "c", "d", "e", "f", "g"},
			},
			query: "test",
			validate: func(t *testing.T, p *QueryPlan) {
				if len(p.Synonyms) != 5 {
					t.Errorf("expected 5 synonyms, got %d", len(p.Synonyms))
				}
			},
		},
		{
			name: "fix invalid intent",
			plan: &QueryPlan{
				Intent: "invalid",
			},
			query: "test",
			validate: func(t *testing.T, p *QueryPlan) {
				if p.Intent != IntentSemantic {
					t.Errorf("expected intent to be semantic, got %s", p.Intent)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := svc.validateAndFix(tt.plan, tt.query)
			tt.validate(t, result)
		})
	}
}

func TestStreamParser(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedJSON   string
		expectComplete bool
	}{
		{
			name: "simple think and json",
			input: `<think>
用户想找无语的表情包
</think>
{"intent":"emotion","semantic_query":"无语表情包","keywords":["无语"],"strategy":{"dense_weight":0.8}}`,
			expectedJSON:   `{"intent":"emotion","semantic_query":"无语表情包","keywords":["无语"],"strategy":{"dense_weight":0.8}}`,
			expectComplete: true,
		},
		{
			name:           "json only",
			input:          `{"intent":"emotion","semantic_query":"test","keywords":[],"strategy":{"dense_weight":0.5}}`,
			expectedJSON:   `{"intent":"emotion","semantic_query":"test","keywords":[],"strategy":{"dense_weight":0.5}}`,
			expectComplete: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := newStreamParser()

			var complete bool
			// Feed character by character to simulate streaming
			for _, char := range tt.input {
				_, c := parser.Feed(string(char))
				if c {
					complete = true
				}
			}

			if complete != tt.expectComplete {
				t.Errorf("expected complete=%v, got %v", tt.expectComplete, complete)
			}

			json := parser.GetJSON()
			if json != tt.expectedJSON {
				t.Errorf("expected JSON:\n%s\ngot:\n%s", tt.expectedJSON, json)
			}
		})
	}
}

func TestQueryPlanCache(t *testing.T) {
	cache := newQueryPlanCache(3, 100*time.Millisecond)

	// Test Set and Get
	plan1 := &QueryPlan{Intent: IntentEmotion, SemanticQuery: "test1"}
	plan2 := &QueryPlan{Intent: IntentSubject, SemanticQuery: "test2"}
	plan3 := &QueryPlan{Intent: IntentMeme, SemanticQuery: "test3"}

	cache.Set("query1", plan1)
	cache.Set("query2", plan2)
	cache.Set("query3", plan3)

	// Verify all are retrievable
	if p, ok := cache.Get("query1"); !ok || p.Intent != IntentEmotion {
		t.Error("failed to get query1")
	}
	if p, ok := cache.Get("query2"); !ok || p.Intent != IntentSubject {
		t.Error("failed to get query2")
	}
	if p, ok := cache.Get("query3"); !ok || p.Intent != IntentMeme {
		t.Error("failed to get query3")
	}

	// Test LRU eviction
	plan4 := &QueryPlan{Intent: IntentScene, SemanticQuery: "test4"}
	cache.Set("query4", plan4)

	// query1 should be evicted (oldest)
	if _, ok := cache.Get("query1"); ok {
		t.Error("query1 should have been evicted")
	}

	// Others should still exist
	if _, ok := cache.Get("query2"); !ok {
		t.Error("query2 should still exist")
	}

	// Test TTL expiration
	time.Sleep(150 * time.Millisecond)
	if _, ok := cache.Get("query2"); ok {
		t.Error("query2 should have expired")
	}
}

func TestQueryPlanCache_Normalization(t *testing.T) {
	cache := newQueryPlanCache(10, time.Hour)

	plan := &QueryPlan{Intent: IntentEmotion, SemanticQuery: "test"}
	cache.Set("  Test Query  ", plan)

	// Should match with different cases and whitespace
	if _, ok := cache.Get("test query"); !ok {
		t.Error("should find with lowercase")
	}
	if _, ok := cache.Get("TEST QUERY"); !ok {
		t.Error("should find with uppercase")
	}
	if _, ok := cache.Get("  test query  "); !ok {
		t.Error("should find with extra whitespace")
	}
}

func TestBuildHybridPlanFromQueryPlan(t *testing.T) {
	tests := []struct {
		name              string
		denseWeight       float32
		topK              int
		expectedDenseMin  int
		expectedDenseMax  int
		expectedSparseMin int
		expectedSparseMax int
	}{
		{
			name:              "high dense weight (0.8)",
			denseWeight:       0.8,
			topK:              20,
			expectedDenseMin:  60, // topK * 3.4
			expectedDenseMax:  80, // topK * 4
			expectedSparseMin: 20, // topK * 1
			expectedSparseMax: 40, // topK * 2
		},
		{
			name:              "balanced (0.5)",
			denseWeight:       0.5,
			topK:              20,
			expectedDenseMin:  40, // topK * 2
			expectedDenseMax:  60, // topK * 3
			expectedSparseMin: 40, // topK * 2
			expectedSparseMax: 60, // topK * 3
		},
		{
			name:              "low dense weight (0.3)",
			denseWeight:       0.3,
			topK:              20,
			expectedDenseMin:  30, // topK * 1.5
			expectedDenseMax:  50, // topK * 2.5
			expectedSparseMin: 50, // topK * 2.5
			expectedSparseMax: 70, // topK * 3.5
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queryPlan := &QueryPlan{
				Strategy: SearchStrategy{
					DenseWeight: tt.denseWeight,
				},
			}

			plan := buildHybridPlanFromQueryPlan(queryPlan, tt.topK)

			if plan.DenseLimit < tt.expectedDenseMin || plan.DenseLimit > tt.expectedDenseMax {
				t.Errorf("DenseLimit %d outside expected range [%d, %d]",
					plan.DenseLimit, tt.expectedDenseMin, tt.expectedDenseMax)
			}

			if plan.SparseLimit < tt.expectedSparseMin || plan.SparseLimit > tt.expectedSparseMax {
				t.Errorf("SparseLimit %d outside expected range [%d, %d]",
					plan.SparseLimit, tt.expectedSparseMin, tt.expectedSparseMax)
			}

			if !plan.UseDense {
				t.Error("expected UseDense to be true")
			}
			if !plan.UseSparse {
				t.Error("expected UseSparse to be true")
			}
		})
	}
}

func TestBuildBM25QueryFromPlan(t *testing.T) {
	tests := []struct {
		name          string
		plan          *QueryPlan
		expectedTerms []string
		excludedTerms []string
	}{
		{
			name: "keywords only with exact match",
			plan: &QueryPlan{
				Keywords: []string{"熊猫头", "无语"},
				Synonyms: []string{"嫌弃", "翻白眼"},
				Strategy: SearchStrategy{NeedExactMatch: true},
			},
			expectedTerms: []string{"熊猫头", "无语"},
			excludedTerms: []string{"嫌弃", "翻白眼"},
		},
		{
			name: "keywords and synonyms",
			plan: &QueryPlan{
				Keywords: []string{"熊猫头", "无语"},
				Synonyms: []string{"嫌弃", "翻白眼"},
				Strategy: SearchStrategy{NeedExactMatch: false},
			},
			expectedTerms: []string{"熊猫头", "无语", "嫌弃", "翻白眼"},
			excludedTerms: []string{},
		},
		{
			name: "deduplication",
			plan: &QueryPlan{
				Keywords: []string{"无语", "无语"},
				Synonyms: []string{"无语"},
				Strategy: SearchStrategy{NeedExactMatch: false},
			},
			expectedTerms: []string{"无语"},
			excludedTerms: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildBM25QueryFromPlan(tt.plan)

			for _, term := range tt.expectedTerms {
				if !strings.Contains(result, term) {
					t.Errorf("expected result to contain %q, got %q", term, result)
				}
			}

			for _, term := range tt.excludedTerms {
				if strings.Contains(result, term) {
					t.Errorf("expected result to NOT contain %q, got %q", term, result)
				}
			}
		})
	}
}

func TestHasQuote(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{`"hello"`, true},
		{`'hello'`, true},
		{`"hello"`, true},
		{`'hello'`, true},
		{`「hello」`, true},
		{`『hello』`, true},
		{`hello`, false},
		{`hello world`, false},
	}

	for _, tt := range tests {
		if got := hasQuote(tt.input); got != tt.expected {
			t.Errorf("hasQuote(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestContainsDigit(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"hello123", true},
		{"666", true},
		{"hello", false},
		{"", false},
		{"hello world", false},
	}

	for _, tt := range tests {
		if got := containsDigit(tt.input); got != tt.expected {
			t.Errorf("containsDigit(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}
