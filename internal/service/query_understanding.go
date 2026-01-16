package service

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/timmy/emomo/internal/prompts"
)

// QueryIntent represents the type of user query intent.
type QueryIntent string

const (
	IntentEmotion   QueryIntent = "emotion"   // 情绪表达 (无语、开心、emo)
	IntentMeme      QueryIntent = "meme"      // 网络流行梗 (芭比Q、绝绝子)
	IntentSubject   QueryIntent = "subject"   // 主体/角色 (熊猫头、猫咪)
	IntentScene     QueryIntent = "scene"     // 使用场景 (上班、恋爱)
	IntentAction    QueryIntent = "action"    // 动作描述 (比心、翻白眼)
	IntentText      QueryIntent = "text"      // 文字内容 (有666的、写着谢谢)
	IntentComposite QueryIntent = "composite" // 复合意图 (熊猫头无语)
	IntentSemantic  QueryIntent = "semantic"  // 默认语义 (fallback)
)

// SearchStrategy defines the retrieval strategy for hybrid search.
type SearchStrategy struct {
	DenseWeight    float32 `json:"dense_weight"`     // 0-1, 0=全BM25, 1=全语义
	NeedExactMatch bool    `json:"need_exact_match"` // 是否需要精确匹配
}

// SuggestedFilters represents optional filters suggested by the LLM.
type SuggestedFilters struct {
	Categories []string `json:"categories,omitempty"`
	IsAnimated *bool    `json:"is_animated,omitempty"`
}

// QueryPlan is the structured output from query understanding.
type QueryPlan struct {
	Intent        QueryIntent       `json:"intent"`
	SemanticQuery string            `json:"semantic_query"`
	Keywords      []string          `json:"keywords"`
	Synonyms      []string          `json:"synonyms,omitempty"`
	Strategy      SearchStrategy    `json:"strategy"`
	Filters       *SuggestedFilters `json:"filters,omitempty"`
	Reasoning     string            `json:"-"` // LLM思考过程，内部使用
}

// UnderstandProgress represents a progress update during streaming understanding.
type UnderstandProgress struct {
	Stage        string `json:"stage"`                   // "thinking" | "parsing" | "done"
	ThinkingText string `json:"thinking_text,omitempty"` // 思考内容 (增量)
	IsDelta      bool   `json:"is_delta,omitempty"`      // 是否为增量更新
	Message      string `json:"message,omitempty"`       // 用户友好消息
}

// QueryUnderstandingConfig holds configuration for query understanding service.
type QueryUnderstandingConfig struct {
	Enabled   bool
	Model     string
	APIKey    string
	BaseURL   string
	CacheSize int           // LRU cache size
	CacheTTL  time.Duration // Cache TTL
}

// QueryUnderstandingService handles query understanding using LLM.
type QueryUnderstandingService struct {
	client   *resty.Client
	model    string
	endpoint string
	apiKey   string
	enabled  bool
	cache    *queryPlanCache
}

// NewQueryUnderstandingService creates a new query understanding service.
func NewQueryUnderstandingService(cfg *QueryUnderstandingConfig) *QueryUnderstandingService {
	if cfg == nil || !cfg.Enabled {
		return &QueryUnderstandingService{enabled: false}
	}

	client := resty.New()
	client.SetHeader("Authorization", "Bearer "+cfg.APIKey)
	client.SetHeader("Content-Type", "application/json")
	client.SetTimeout(30 * time.Second)

	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	endpoint := baseURL + "/chat/completions"

	cacheSize := cfg.CacheSize
	if cacheSize <= 0 {
		cacheSize = 100
	}
	cacheTTL := cfg.CacheTTL
	if cacheTTL <= 0 {
		cacheTTL = 10 * time.Minute
	}

	return &QueryUnderstandingService{
		client:   client,
		model:    cfg.Model,
		endpoint: endpoint,
		apiKey:   cfg.APIKey,
		enabled:  true,
		cache:    newQueryPlanCache(cacheSize, cacheTTL),
	}
}

// IsEnabled returns whether query understanding is enabled.
func (s *QueryUnderstandingService) IsEnabled() bool {
	return s.enabled
}

// llmRequest represents the request to the LLM API.
type llmRequest struct {
	Model       string       `json:"model"`
	Messages    []llmMessage `json:"messages"`
	MaxTokens   int          `json:"max_tokens"`
	Temperature float32      `json:"temperature"`
	Stream      bool         `json:"stream,omitempty"`
}

type llmMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type llmResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type streamDelta struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}

// Understand performs query understanding and returns a structured QueryPlan.
func (s *QueryUnderstandingService) Understand(ctx context.Context, query string) (*QueryPlan, error) {
	if !s.enabled {
		return s.fallbackUnderstand(query), nil
	}

	// Check cache
	if cached, ok := s.cache.Get(query); ok {
		return cached, nil
	}

	// Skip LLM for already long queries (likely already descriptive)
	if len([]rune(query)) > 100 {
		plan := s.fallbackUnderstand(query)
		return plan, nil
	}

	req := llmRequest{
		Model: s.model,
		Messages: []llmMessage{
			{Role: "system", Content: prompts.QueryUnderstandingPrompt},
			{Role: "user", Content: query},
		},
		MaxTokens:   300,
		Temperature: 0.3,
	}

	var resp llmResponse
	httpResp, err := s.client.R().
		SetContext(ctx).
		SetBody(req).
		SetResult(&resp).
		Post(s.endpoint)

	if err != nil {
		return s.fallbackUnderstand(query), nil
	}

	if httpResp.StatusCode() < 200 || httpResp.StatusCode() >= 300 {
		return s.fallbackUnderstand(query), nil
	}

	if len(resp.Choices) == 0 || resp.Choices[0].Message.Content == "" {
		return s.fallbackUnderstand(query), nil
	}

	content := resp.Choices[0].Message.Content
	plan, err := s.parseResponse(content, query)
	if err != nil {
		return s.fallbackUnderstand(query), nil
	}

	// Cache the result
	s.cache.Set(query, plan)

	return plan, nil
}

// UnderstandStream performs streaming query understanding.
func (s *QueryUnderstandingService) UnderstandStream(
	ctx context.Context,
	query string,
	progressCh chan<- UnderstandProgress,
) (*QueryPlan, error) {
	defer close(progressCh)

	if !s.enabled {
		return s.fallbackUnderstand(query), nil
	}

	// Check cache
	if cached, ok := s.cache.Get(query); ok {
		progressCh <- UnderstandProgress{
			Stage:   "done",
			Message: "理解完成（缓存）",
		}
		return cached, nil
	}

	// Skip LLM for long queries
	if len([]rune(query)) > 100 {
		return s.fallbackUnderstand(query), nil
	}

	// Send start event
	progressCh <- UnderstandProgress{
		Stage:   "thinking_start",
		Message: "AI 正在理解搜索意图...",
	}

	req := llmRequest{
		Model: s.model,
		Messages: []llmMessage{
			{Role: "system", Content: prompts.QueryUnderstandingPrompt},
			{Role: "user", Content: query},
		},
		MaxTokens:   300,
		Temperature: 0.3,
		Stream:      true,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return s.fallbackUnderstand(query), nil
	}

	// Create HTTP request for streaming
	httpReq, err := http.NewRequestWithContext(ctx, "POST", s.endpoint, strings.NewReader(string(reqBody)))
	if err != nil {
		return s.fallbackUnderstand(query), nil
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+s.apiKey)
	httpReq.Header.Set("Accept", "text/event-stream")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return s.fallbackUnderstand(query), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return s.fallbackUnderstand(query), nil
	}

	// Parse SSE stream with StreamParser
	parser := newStreamParser()
	scanner := bufio.NewScanner(resp.Body)

	for scanner.Scan() {
		line := scanner.Text()

		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}

		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				break
			}

			var delta streamDelta
			if err := json.Unmarshal([]byte(data), &delta); err != nil {
				continue
			}

			if len(delta.Choices) == 0 {
				continue
			}

			token := delta.Choices[0].Delta.Content
			thinkingToken, isComplete := parser.Feed(token)

			// Send thinking content
			if thinkingToken != "" {
				progressCh <- UnderstandProgress{
					Stage:        "thinking",
					ThinkingText: thinkingToken,
					IsDelta:      true,
				}
			}

			if isComplete {
				break
			}
		}
	}

	// Parse JSON
	jsonStr := parser.GetJSON()
	if jsonStr == "" {
		return s.fallbackUnderstand(query), nil
	}

	var plan QueryPlan
	if err := json.Unmarshal([]byte(jsonStr), &plan); err != nil {
		return s.fallbackUnderstand(query), nil
	}

	// Store thinking for debugging
	plan.Reasoning = parser.GetThinking()

	// Validate and fix
	plan = *s.validateAndFix(&plan, query)

	// Cache the result
	s.cache.Set(query, &plan)

	// Send done event
	progressCh <- UnderstandProgress{
		Stage:   "done",
		Message: "理解完成",
	}

	return &plan, nil
}

// parseResponse parses the LLM response and extracts the QueryPlan.
func (s *QueryUnderstandingService) parseResponse(content, originalQuery string) (*QueryPlan, error) {
	// Extract thinking content (for logging/debugging)
	thinking := ""
	if start := strings.Index(content, "<think>"); start != -1 {
		if end := strings.Index(content, "</think>"); end != -1 {
			thinking = strings.TrimSpace(content[start+7 : end])
			content = content[end+8:]
		}
	}

	// Find JSON start
	jsonStart := strings.Index(content, "{")
	if jsonStart == -1 {
		return nil, fmt.Errorf("no JSON found in response")
	}

	// Find matching closing brace
	braceCount := 0
	jsonEnd := -1
findJSON:
	for i := jsonStart; i < len(content); i++ {
		switch content[i] {
		case '{':
			braceCount++
		case '}':
			braceCount--
			if braceCount == 0 {
				jsonEnd = i + 1
				break findJSON
			}
		}
	}

	if jsonEnd == -1 {
		return nil, fmt.Errorf("incomplete JSON in response")
	}

	jsonStr := content[jsonStart:jsonEnd]

	var plan QueryPlan
	if err := json.Unmarshal([]byte(jsonStr), &plan); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	plan.Reasoning = thinking

	// Validate and fix
	return s.validateAndFix(&plan, originalQuery), nil
}

// fallbackUnderstand provides rule-based understanding when LLM is unavailable.
func (s *QueryUnderstandingService) fallbackUnderstand(query string) *QueryPlan {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return &QueryPlan{
			Intent:        IntentSemantic,
			SemanticQuery: query,
			Keywords:      []string{query},
			Strategy: SearchStrategy{
				DenseWeight:    0.7,
				NeedExactMatch: false,
			},
		}
	}

	intent := IntentSemantic
	denseWeight := float32(0.7)
	var filters *SuggestedFilters

	// Check for quotes or digits -> text intent
	if hasQuote(trimmed) || containsDigit(trimmed) {
		intent = IntentText
		denseWeight = 0.3
	} else {
		// Check emotion words
		hasEmotion := false
		for _, word := range prompts.EmotionWords {
			if word == "" {
				continue
			}
			if strings.Contains(strings.ToLower(trimmed), strings.ToLower(word)) {
				hasEmotion = true
				break
			}
		}

		// Check internet memes
		hasMeme := false
		for _, word := range prompts.InternetMemes {
			if word == "" {
				continue
			}
			if strings.Contains(strings.ToLower(trimmed), strings.ToLower(word)) {
				hasMeme = true
				break
			}
		}

		// Check subject keywords
		subjects := []string{"熊猫头", "蘑菇头", "猫咪", "柴犬", "兔子", "小黄人", "派大星", "海绵宝宝"}
		hasSubject := false
		var matchedSubject string
		for _, subject := range subjects {
			if strings.Contains(trimmed, subject) {
				hasSubject = true
				matchedSubject = subject
				break
			}
		}

		// Determine intent based on detected patterns
		if hasSubject && hasEmotion {
			intent = IntentComposite
			denseWeight = 0.6
			filters = &SuggestedFilters{Categories: []string{matchedSubject}}
		} else if hasSubject {
			intent = IntentSubject
			denseWeight = 0.5
			filters = &SuggestedFilters{Categories: []string{matchedSubject}}
		} else if hasMeme {
			intent = IntentMeme
			denseWeight = 0.6
		} else if hasEmotion {
			intent = IntentEmotion
			denseWeight = 0.8
		} else if len([]rune(trimmed)) <= 6 {
			// Short query - treat as subject/exact
			intent = IntentSubject
			denseWeight = 0.5
		}
	}

	return &QueryPlan{
		Intent:        intent,
		SemanticQuery: query, // Use original query
		Keywords:      []string{query},
		Strategy: SearchStrategy{
			DenseWeight:    denseWeight,
			NeedExactMatch: len([]rune(query)) <= 4,
		},
		Filters: filters,
	}
}

// validateAndFix validates and fixes the QueryPlan.
func (s *QueryUnderstandingService) validateAndFix(plan *QueryPlan, originalQuery string) *QueryPlan {
	// 1. Validate Intent
	validIntents := map[QueryIntent]bool{
		IntentEmotion: true, IntentMeme: true, IntentSubject: true,
		IntentScene: true, IntentAction: true, IntentText: true,
		IntentComposite: true, IntentSemantic: true,
	}
	if !validIntents[plan.Intent] {
		plan.Intent = IntentSemantic
	}

	// 2. Validate DenseWeight range
	if plan.Strategy.DenseWeight < 0 {
		plan.Strategy.DenseWeight = 0
	}
	if plan.Strategy.DenseWeight > 1 {
		plan.Strategy.DenseWeight = 1
	}

	// 3. Validate SemanticQuery length
	if len([]rune(plan.SemanticQuery)) < 10 {
		plan.SemanticQuery = originalQuery
	}
	if len([]rune(plan.SemanticQuery)) > 200 {
		plan.SemanticQuery = string([]rune(plan.SemanticQuery)[:200])
	}

	// 4. Validate Keywords
	if len(plan.Keywords) == 0 {
		plan.Keywords = []string{originalQuery}
	}
	if len(plan.Keywords) > 5 {
		plan.Keywords = plan.Keywords[:5]
	}

	// 5. Validate Synonyms
	if len(plan.Synonyms) > 5 {
		plan.Synonyms = plan.Synonyms[:5]
	}

	return plan
}

// hasQuote checks if the text contains quote characters.
func hasQuote(text string) bool {
	quoteChars := "\"'" + "\u201c\u201d\u2018\u2019\u300c\u300d\u300e\u300f"
	return strings.ContainsAny(text, quoteChars)
}

// containsDigit checks if the text contains any digit.
func containsDigit(text string) bool {
	for _, r := range text {
		if r >= '0' && r <= '9' {
			return true
		}
	}
	return false
}

// ============================================================================
// StreamParser - State machine for parsing LLM streaming output
// ============================================================================

type parseState int

const (
	stateStart parseState = iota
	stateThinking
	stateAwaitingJSON
	stateParsingJSON
	stateComplete
)

// streamParser parses LLM streaming output.
type streamParser struct {
	state        parseState
	thinkBuffer  strings.Builder
	jsonBuffer   strings.Builder
	braceCount   int
	pendingStart string // Buffer for detecting <think> tag
}

func newStreamParser() *streamParser {
	return &streamParser{
		state: stateStart,
	}
}

// Feed processes a token and returns (thinkingToken, isComplete).
func (p *streamParser) Feed(token string) (string, bool) {
	var thinkingOutput strings.Builder

	for _, char := range token {
		switch p.state {
		case stateStart:
			// Check if JSON starts directly
			if char == '{' {
				p.jsonBuffer.WriteRune(char)
				p.braceCount = 1
				p.state = stateParsingJSON
				continue
			}

			p.pendingStart += string(char)
			if strings.HasSuffix(p.pendingStart, "<think>") {
				p.state = stateThinking
				p.pendingStart = ""
			} else if len(p.pendingStart) > 10 && !strings.Contains("<think>", p.pendingStart) {
				// Not a think tag, move to JSON state
				p.state = stateAwaitingJSON
			}

		case stateThinking:
			p.thinkBuffer.WriteRune(char)
			content := p.thinkBuffer.String()
			if strings.HasSuffix(content, "</think>") {
				// Remove the closing tag from thinking content
				thinkContent := strings.TrimSuffix(content, "</think>")
				thinkingOutput.WriteString(thinkContent)
				p.thinkBuffer.Reset()
				p.thinkBuffer.WriteString(thinkContent) // Keep for GetThinking()
				p.state = stateAwaitingJSON
			}

		case stateAwaitingJSON:
			if char == '{' {
				p.jsonBuffer.WriteRune(char)
				p.braceCount = 1
				p.state = stateParsingJSON
			}

		case stateParsingJSON:
			p.jsonBuffer.WriteRune(char)
			if char == '{' {
				p.braceCount++
			} else if char == '}' {
				p.braceCount--
				if p.braceCount == 0 {
					p.state = stateComplete
					return thinkingOutput.String(), true
				}
			}
		}
	}

	// Return accumulated thinking content if in thinking state
	if p.state == stateThinking {
		// For streaming, return the token as-is for real-time display
		return token, false
	}

	return thinkingOutput.String(), false
}

// GetJSON returns the parsed JSON string.
func (p *streamParser) GetJSON() string {
	return p.jsonBuffer.String()
}

// GetThinking returns the thinking content.
func (p *streamParser) GetThinking() string {
	return p.thinkBuffer.String()
}

// ============================================================================
// QueryPlanCache - LRU cache with TTL
// ============================================================================

type cachedPlan struct {
	plan      *QueryPlan
	timestamp time.Time
}

type queryPlanCache struct {
	mu      sync.RWMutex
	cache   map[string]*cachedPlan
	ttl     time.Duration
	maxSize int
	order   []string // LRU order (oldest first)
}

func newQueryPlanCache(maxSize int, ttl time.Duration) *queryPlanCache {
	return &queryPlanCache{
		cache:   make(map[string]*cachedPlan),
		ttl:     ttl,
		maxSize: maxSize,
		order:   make([]string, 0, maxSize),
	}
}

func (c *queryPlanCache) Get(query string) (*QueryPlan, bool) {
	key := normalizeQuery(query)

	c.mu.RLock()
	cached, ok := c.cache[key]
	c.mu.RUnlock()

	if !ok {
		return nil, false
	}

	if time.Since(cached.timestamp) > c.ttl {
		c.mu.Lock()
		delete(c.cache, key)
		c.removeFromOrder(key)
		c.mu.Unlock()
		return nil, false
	}

	// Move to end of order (most recently used)
	c.mu.Lock()
	c.removeFromOrder(key)
	c.order = append(c.order, key)
	c.mu.Unlock()

	return cached.plan, true
}

func (c *queryPlanCache) Set(query string, plan *QueryPlan) {
	key := normalizeQuery(query)

	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict oldest if at capacity
	for len(c.cache) >= c.maxSize && len(c.order) > 0 {
		oldestKey := c.order[0]
		delete(c.cache, oldestKey)
		c.order = c.order[1:]
	}

	c.cache[key] = &cachedPlan{
		plan:      plan,
		timestamp: time.Now(),
	}

	c.removeFromOrder(key)
	c.order = append(c.order, key)
}

func (c *queryPlanCache) removeFromOrder(key string) {
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			break
		}
	}
}

func normalizeQuery(query string) string {
	return strings.ToLower(strings.TrimSpace(query))
}
