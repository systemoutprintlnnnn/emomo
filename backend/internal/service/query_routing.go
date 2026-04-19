package service

import (
	"strings"
	"unicode"

	"github.com/timmy/emomo/internal/repository"
)

type QueryRoute string

const (
	QueryRouteExact    QueryRoute = "exact"
	QueryRouteEmotion  QueryRoute = "emotion"
	QueryRouteSemantic QueryRoute = "semantic"
)

const (
	shortQueryMaxRunes = 6
	maxPrefetchLimit   = 200
	defaultRRFK        = 60
	exactDenseBoost    = 1
	exactSparseBoost   = 3
	emotionDenseBoost  = 3
	semanticDenseBoost = 3
	defaultSparseBoost = 2
)

func classifyQuery(query string) QueryRoute {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return QueryRouteSemantic
	}

	if hasQuote(trimmed) || containsDigit(trimmed) {
		return QueryRouteExact
	}

	if containsIntentKeyword(trimmed) {
		return QueryRouteEmotion
	}

	if runeCount(trimmed) <= shortQueryMaxRunes {
		return QueryRouteExact
	}

	return QueryRouteSemantic
}

func buildHybridPlan(route QueryRoute, topK int) repository.HybridSearchPlan {
	if topK <= 0 {
		topK = 20
	}

	plan := repository.HybridSearchPlan{
		UseDense:  true,
		UseSparse: true,
		RRFK:      defaultRRFK,
	}

	switch route {
	case QueryRouteExact:
		plan.DenseLimit = clampPrefetch(topK * exactDenseBoost)
		plan.SparseLimit = clampPrefetch(topK * exactSparseBoost)
	case QueryRouteEmotion:
		plan.DenseLimit = clampPrefetch(topK * emotionDenseBoost)
		plan.SparseLimit = clampPrefetch(topK * defaultSparseBoost)
	default:
		plan.DenseLimit = clampPrefetch(topK * semanticDenseBoost)
		plan.SparseLimit = clampPrefetch(topK * defaultSparseBoost)
	}

	return plan
}

func clampPrefetch(limit int) int {
	if limit <= 0 {
		return 1
	}
	if limit > maxPrefetchLimit {
		return maxPrefetchLimit
	}
	return limit
}

func runeCount(text string) int {
	return len([]rune(text))
}

func hasQuote(text string) bool {
	return strings.ContainsAny(text, "\"'“”‘’「」『』")
}

func containsDigit(text string) bool {
	for _, r := range text {
		if unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

func containsIntentKeyword(text string) bool {
	lower := strings.ToLower(text)
	for _, word := range EmotionWords {
		if word == "" {
			continue
		}
		if strings.Contains(lower, strings.ToLower(word)) {
			return true
		}
	}
	for _, word := range InternetMemes {
		if word == "" {
			continue
		}
		if strings.Contains(lower, strings.ToLower(word)) {
			return true
		}
	}
	return false
}
