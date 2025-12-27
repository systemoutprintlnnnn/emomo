package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/go-resty/resty/v2"
)

const (
	vlmPrompt = `请用简洁的中文描述这张表情包的内容，包括：
1. 图片中的主体（人物、动物、卡通形象等）
2. 表情或动作
3. 图片上的文字（如果有）
4. 表达的情绪或场景
输出格式：一段 50-100 字的描述，不要使用序号或分点。`
)

// VLMService handles image description generation using Vision Language Models
type VLMService struct {
	client   *resty.Client
	model    string
	apiKey   string
	endpoint string
}

// VLMConfig holds configuration for VLM service
type VLMConfig struct {
	Provider string
	Model    string
	APIKey   string
	BaseURL  string
}

// NewVLMService creates a new VLM service
func NewVLMService(cfg *VLMConfig) *VLMService {
	client := resty.New()
	client.SetHeader("Authorization", "Bearer "+cfg.APIKey)
	client.SetHeader("Content-Type", "application/json")
	// Set timeout to prevent hanging requests
	client.SetTimeout(60 * time.Second)

	// Default to OpenAI if not specified
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	endpoint := baseURL + "/chat/completions"

	return &VLMService{
		client:   client,
		model:    cfg.Model,
		apiKey:   cfg.APIKey,
		endpoint: endpoint,
	}
}

// GetModel returns the model name being used
func (s *VLMService) GetModel() string {
	return s.model
}

// OpenAI API request/response structures
type openAIRequest struct {
	Model     string          `json:"model"`
	Messages  []openAIMessage `json:"messages"`
	MaxTokens int             `json:"max_tokens"`
}

type openAIMessage struct {
	Role    string        `json:"role"`
	Content []interface{} `json:"content"`
}

type openAITextContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type openAIImageContent struct {
	Type     string         `json:"type"`
	ImageURL openAIImageURL `json:"image_url"`
}

type openAIImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

// DescribeImage generates a description for an image
func (s *VLMService) DescribeImage(ctx context.Context, imageData []byte, format string) (string, error) {
	// Determine MIME type
	mimeType := getMIMEType(format)

	// Encode image to base64
	base64Image := base64.StdEncoding.EncodeToString(imageData)
	dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, base64Image)

	// Build request
	req := openAIRequest{
		Model: s.model,
		Messages: []openAIMessage{
			{
				Role: "user",
				Content: []interface{}{
					openAITextContent{
						Type: "text",
						Text: vlmPrompt,
					},
					openAIImageContent{
						Type: "image_url",
						ImageURL: openAIImageURL{
							URL:    dataURL,
							Detail: "low", // Use low detail to reduce cost
						},
					},
				},
			},
		},
		MaxTokens: 300,
	}

	// Send request
	var resp openAIResponse
	httpResp, err := s.client.R().
		SetContext(ctx).
		SetBody(req).
		SetResult(&resp).
		Post(s.endpoint)

	if err != nil {
		return "", fmt.Errorf("failed to call VLM API: %w", err)
	}

	// Check HTTP status code
	if httpResp.StatusCode() < 200 || httpResp.StatusCode() >= 300 {
		// Try to get error message from response body
		errorMsg := fmt.Sprintf("HTTP %d", httpResp.StatusCode())
		if resp.Error != nil {
			errorMsg = fmt.Sprintf("HTTP %d: %s", httpResp.StatusCode(), resp.Error.Message)
		} else {
			// Include response body for debugging
			errorMsg = fmt.Sprintf("HTTP %d: %s", httpResp.StatusCode(), string(httpResp.Body()))
		}
		return "", fmt.Errorf("VLM API returned error: %s", errorMsg)
	}

	if resp.Error != nil {
		return "", fmt.Errorf("VLM API error: %s", resp.Error.Message)
	}

	if len(resp.Choices) == 0 {
		// Include more context in error message
		errorMsg := fmt.Sprintf("no choices in response (status: %d)", httpResp.StatusCode())
		if len(httpResp.Body()) > 0 {
			errorMsg += fmt.Sprintf(", response body: %s", string(httpResp.Body()))
		}
		return "", fmt.Errorf("no response from VLM API: %s", errorMsg)
	}

	return resp.Choices[0].Message.Content, nil
}

// DescribeImageFromURL generates a description for an image from URL
func (s *VLMService) DescribeImageFromURL(ctx context.Context, imageURL string) (string, error) {
	req := openAIRequest{
		Model: s.model,
		Messages: []openAIMessage{
			{
				Role: "user",
				Content: []interface{}{
					openAITextContent{
						Type: "text",
						Text: vlmPrompt,
					},
					openAIImageContent{
						Type: "image_url",
						ImageURL: openAIImageURL{
							URL:    imageURL,
							Detail: "low",
						},
					},
				},
			},
		},
		MaxTokens: 300,
	}

	var resp openAIResponse
	httpResp, err := s.client.R().
		SetContext(ctx).
		SetBody(req).
		SetResult(&resp).
		Post(s.endpoint)

	if err != nil {
		return "", fmt.Errorf("failed to call VLM API: %w", err)
	}

	// Check HTTP status code
	if httpResp.StatusCode() < 200 || httpResp.StatusCode() >= 300 {
		// Try to get error message from response body
		errorMsg := fmt.Sprintf("HTTP %d", httpResp.StatusCode())
		if resp.Error != nil {
			errorMsg = fmt.Sprintf("HTTP %d: %s", httpResp.StatusCode(), resp.Error.Message)
		} else {
			// Include response body for debugging
			errorMsg = fmt.Sprintf("HTTP %d: %s", httpResp.StatusCode(), string(httpResp.Body()))
		}
		return "", fmt.Errorf("VLM API returned error: %s", errorMsg)
	}

	if resp.Error != nil {
		return "", fmt.Errorf("VLM API error: %s", resp.Error.Message)
	}

	if len(resp.Choices) == 0 {
		// Include more context in error message
		errorMsg := fmt.Sprintf("no choices in response (status: %d)", httpResp.StatusCode())
		if len(httpResp.Body()) > 0 {
			errorMsg += fmt.Sprintf(", response body: %s", string(httpResp.Body()))
		}
		return "", fmt.Errorf("no response from VLM API: %s", errorMsg)
	}

	return resp.Choices[0].Message.Content, nil
}

func getMIMEType(format string) string {
	switch format {
	case "jpg", "jpeg":
		return "image/jpeg"
	case "png":
		return "image/png"
	case "gif":
		return "image/gif"
	case "webp":
		return "image/webp"
	default:
		return "image/jpeg"
	}
}
