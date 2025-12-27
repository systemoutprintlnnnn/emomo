package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
)

const (
	queryExpansionPrompt = `你是一个表情包搜索查询扩展专家。用户会输入简短的搜索词，你需要将其扩展为更丰富的语义描述，以便更好地匹配表情包的描述文本。

【扩展规则】
1. 保留用户原始查询的核心意图
2. 添加相关的情绪词、同义词、近义词
3. 描述可能对应的表情包画面场景
4. 如果是网络流行语/梗，解释其含义并添加相关情绪
5. 输出应该是一段自然的描述性文本，50-80字

【常见情绪词参考】
无语、尴尬、开心、暴怒、委屈、嫌弃、震惊、疑惑、得意、摆烂、emo、社死、破防、裂开、绝望、狂喜、阴阳怪气、幸灾乐祸、无奈、崩溃、感动、害怕、可爱、呆萌

【网络梗参考】
芭比Q了(完蛋了)、绝绝子(太绝了)、yyds(永远的神)、真的栓Q(真的谢谢)、CPU(欺骗PUA)、emo(情绪低落)、摆烂(放弃挣扎)、社死(社会性死亡)、破防(情绪崩溃)

【示例】
用户输入: 无语
扩展输出: 表情包表达无语、无奈、嫌弃的情绪，可能是人物或动物露出白眼、翻白眼、面无表情、一脸嫌弃的样子，表示对某事感到无话可说或不想理会

用户输入: 开心
扩展输出: 表情包表达开心、快乐、高兴的情绪，可能是人物或动物笑得很开心、手舞足蹈、眉开眼笑、欢呼雀跃的样子，表示非常愉悦满足

用户输入: 芭比Q了
扩展输出: 表情包表达完蛋了、糟糕了、大事不妙的情绪，芭比Q是网络流行语表示完蛋，可能是人物或动物露出惊恐、绝望、崩溃的表情，表示事情搞砸了

用户输入: 好累啊
扩展输出: 表情包表达疲惫、累了、困倦、想休息的情绪，可能是人物或动物瘫倒、趴着、无精打采、眼睛半闭的样子，表示身心俱疲想要摆烂

现在请扩展以下查询，只输出扩展后的文本，不要有任何前缀或解释：`
)

// QueryExpansionService handles query expansion using LLM
type QueryExpansionService struct {
	client   *resty.Client
	model    string
	endpoint string
	enabled  bool
}

// QueryExpansionConfig holds configuration for query expansion service
type QueryExpansionConfig struct {
	Enabled bool
	Model   string
	APIKey  string
	BaseURL string
}

// NewQueryExpansionService creates a new query expansion service
func NewQueryExpansionService(cfg *QueryExpansionConfig) *QueryExpansionService {
	if cfg == nil || !cfg.Enabled {
		return &QueryExpansionService{enabled: false}
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

	return &QueryExpansionService{
		client:   client,
		model:    cfg.Model,
		endpoint: endpoint,
		enabled:  true,
	}
}

// IsEnabled returns whether query expansion is enabled
func (s *QueryExpansionService) IsEnabled() bool {
	return s.enabled
}

// queryExpansionRequest represents the request to the LLM API
type queryExpansionRequest struct {
	Model     string                      `json:"model"`
	Messages  []queryExpansionMessage     `json:"messages"`
	MaxTokens int                         `json:"max_tokens"`
	Temperature float32                   `json:"temperature"`
}

type queryExpansionMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type queryExpansionResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Expand expands a short query into a richer semantic description
func (s *QueryExpansionService) Expand(ctx context.Context, query string) (string, error) {
	if !s.enabled {
		return query, nil
	}

	// Skip expansion for already long queries (likely already descriptive)
	if len([]rune(query)) > 50 {
		return query, nil
	}

	req := queryExpansionRequest{
		Model: s.model,
		Messages: []queryExpansionMessage{
			{
				Role:    "system",
				Content: queryExpansionPrompt,
			},
			{
				Role:    "user",
				Content: query,
			},
		},
		MaxTokens:   150,
		Temperature: 0.3, // Lower temperature for more consistent expansions
	}

	var resp queryExpansionResponse
	httpResp, err := s.client.R().
		SetContext(ctx).
		SetBody(req).
		SetResult(&resp).
		Post(s.endpoint)

	if err != nil {
		// On error, fall back to original query
		return query, fmt.Errorf("query expansion API call failed: %w", err)
	}

	if httpResp.StatusCode() < 200 || httpResp.StatusCode() >= 300 {
		if resp.Error != nil {
			return query, fmt.Errorf("query expansion API error: %s", resp.Error.Message)
		}
		return query, fmt.Errorf("query expansion API error: status %d", httpResp.StatusCode())
	}

	if len(resp.Choices) == 0 || resp.Choices[0].Message.Content == "" {
		return query, nil
	}

	expanded := strings.TrimSpace(resp.Choices[0].Message.Content)

	// Validate expansion - if it's too short or seems invalid, return original
	if len([]rune(expanded)) < 10 {
		return query, nil
	}

	return expanded, nil
}

// ExpandWithFallback expands query and returns original on any error
func (s *QueryExpansionService) ExpandWithFallback(ctx context.Context, query string) string {
	expanded, err := s.Expand(ctx, query)
	if err != nil {
		return query
	}
	return expanded
}
