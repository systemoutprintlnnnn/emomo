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

// queryUnderstandingPrompt is the system prompt for query understanding.
const queryUnderstandingPrompt = `你是表情包搜索查询理解助手。你的任务是理解用户的搜索意图，并生成结构化的查询计划。

【输出格式】
请严格按照以下格式输出：
1. 先用 <think></think> 标签包裹你的思考过程（2-4 句话，简洁明了）
2. 然后直接输出 JSON（不要用 markdown 代码块）

【意图类型】
- emotion: 情绪表达（无语、开心、emo）
- meme: 网络流行梗（芭比Q、绝绝子、yyds）
- subject: 主体/角色（熊猫头、猫咪、柴犬）
- scene: 使用场景（上班、恋爱、考试）
- action: 动作描述（比心、翻白眼、点赞）
- text: 文字内容（有666的、写着谢谢）
- composite: 复合意图（熊猫头无语 = subject + emotion）

【JSON Schema】
{
  "intent": "emotion|meme|subject|scene|action|text|composite",
  "semantic_query": "50-100字的语义描述，用于向量搜索",
  "keywords": ["关键词1", "关键词2"],  // 最多5个
  "synonyms": ["同义词1", "同义词2"],  // 最多5个，可选
  "strategy": {
    "dense_weight": 0.0-1.0,  // 0=全BM25, 1=全语义
    "need_exact_match": true/false
  },
  "filters": {  // 可选
    "categories": ["熊猫头"]
  }
}

【策略指南】
- text 意图: dense_weight = 0.3 (BM25 为主，搜索 OCR 文本)
- subject 意图: dense_weight = 0.5 (均衡)
- meme 意图: dense_weight = 0.6 (需要理解梗的含义)
- emotion/scene 意图: dense_weight = 0.8 (语义为主)
- composite 意图: dense_weight = 0.5-0.7 (根据具体情况)
- action 意图: dense_weight = 0.7 (语义+关键词结合)

【示例】

输入: 无语
<think>
用户想表达"无语"的情绪，这是典型的 emotion 意图。需要扩展相关情绪词（无奈、嫌弃、翻白眼），并描述这类表情包的视觉特征。语义理解为主，dense_weight 设为 0.8。
</think>
{"intent":"emotion","semantic_query":"无语、无奈、嫌弃的情绪表情包，翻白眼、面无表情、一脸嫌弃的样子，对某事无话可说不想理会，表达对某人某事的无奈和不屑","keywords":["无语","无奈","嫌弃"],"synonyms":["翻白眼","不想说话","懒得理"],"strategy":{"dense_weight":0.8,"need_exact_match":false}}

输入: 熊猫头无语
<think>
查询包含两个部分："熊猫头"是主体类型（subject），"无语"是情绪（emotion），这是复合意图。需要在 filters 中限定熊猫头类别，同时语义描述要结合无语情绪。策略上均衡一些，dense_weight 设为 0.6。
</think>
{"intent":"composite","semantic_query":"熊猫头表情包，表达无语、无奈、嫌弃的情绪，黑白熊猫脸露出一脸嫌弃翻白眼的样子，对某事无话可说","keywords":["熊猫头","无语","无奈"],"synonyms":["嫌弃","翻白眼"],"strategy":{"dense_weight":0.6,"need_exact_match":false},"filters":{"categories":["熊猫头"]}}

输入: 有666的表情包
<think>
用户想找包含"666"文字的表情包，这是 text 意图。需要搜索 OCR 文本，BM25 为主，dense_weight 设为 0.3。关键词就是"666"。
</think>
{"intent":"text","semantic_query":"包含666文字的表情包，写着666、厉害、牛逼的意思，表达赞叹佩服的表情","keywords":["666","厉害","牛"],"synonyms":[],"strategy":{"dense_weight":0.3,"need_exact_match":true}}

输入: 熊猫头
<think>
用户想找熊猫头类型的表情包，这是 subject 意图。需要精确匹配主体类型，同时语义描述熊猫头的特征。策略均衡，dense_weight 设为 0.5。
</think>
{"intent":"subject","semantic_query":"熊猫头表情包，经典黑白熊猫脸，圆圆的脑袋配各种搞怪表情，可表达无语、开心、疑惑、震惊、嫌弃等多种情绪","keywords":["熊猫头"],"synonyms":["熊猫","panda"],"strategy":{"dense_weight":0.5,"need_exact_match":false},"filters":{"categories":["熊猫头"]}}

输入: 芭比Q了
<think>
"芭比Q了"是网络流行梗，意思是"完蛋了"。这是 meme 意图。需要扩展同义词（完蛋、凉了、糟糕），语义描述这类表情的情绪特征。dense_weight 设为 0.6。
</think>
{"intent":"meme","semantic_query":"完蛋了、糟糕了、大事不妙，芭比Q网络流行语表示完蛋，惊恐绝望崩溃的表情，事情搞砸了要完蛋了","keywords":["芭比Q","完蛋"],"synonyms":["凉了","糟糕","完犊子","大事不妙"],"strategy":{"dense_weight":0.6,"need_exact_match":false}}

输入: 比心
<think>
"比心"是一个动作描述，用户想找做比心手势的表情包。这是 action 意图。语义描述比心的动作特征，dense_weight 设为 0.7。
</think>
{"intent":"action","semantic_query":"比心手势表情包，双手比成心形，表达爱意、喜欢、感谢、可爱的表情，爱心手势","keywords":["比心","爱心"],"synonyms":["心形","爱你","么么哒"],"strategy":{"dense_weight":0.7,"need_exact_match":false}}

输入: 上班摸鱼
<think>
"上班摸鱼"描述的是工作场景下偷懒的状态，这是 scene 意图。需要理解场景下的情绪（无聊、偷懒、划水），语义为主，dense_weight 设为 0.8。
</think>
{"intent":"scene","semantic_query":"上班摸鱼划水表情包，工作时间偷懒不想干活，无聊发呆假装很忙实际在摸鱼，打工人摆烂躺平","keywords":["摸鱼","上班","划水"],"synonyms":["偷懒","摆烂","躺平"],"strategy":{"dense_weight":0.8,"need_exact_match":false}}

现在请理解以下查询：`

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
			{Role: "system", Content: queryUnderstandingPrompt},
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
			{Role: "system", Content: queryUnderstandingPrompt},
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
		for _, word := range EmotionWords {
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
		for _, word := range InternetMemes {
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
