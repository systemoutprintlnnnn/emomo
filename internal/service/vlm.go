package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/timmy/emomo/internal/prompts"
)

// VLMService handles image description generation using Vision Language Models.
type VLMService struct {
	client   *resty.Client
	model    string
	apiKey   string
	endpoint string
}

// VLMConfig holds configuration for VLM service.
type VLMConfig struct {
	Provider string
	Model    string
	APIKey   string
	BaseURL  string
}

// NewVLMService creates a new VLM service.
// Parameters:
//   - cfg: VLM configuration including provider, model, and API key.
//
// Returns:
//   - *VLMService: initialized VLM client wrapper.
func NewVLMService(cfg *VLMConfig) *VLMService {
	client := resty.New()
	client.SetHeader("Authorization", "Bearer "+cfg.APIKey)
	client.SetHeader("Content-Type", "application/json")
	// Set timeout to prevent hanging requests
	client.SetTimeout(60 * time.Second)

	// Default to OpenAI compatible endpoint if not specified
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

// GetModel returns the model name being used.
// Parameters: none.
// Returns:
//   - string: model identifier.
func (s *VLMService) GetModel() string {
	return s.model
}

// OpenAI-compatible Chat Completion API request/response structures
type openAIRequest struct {
	Model     string          `json:"model"`
	Messages  []openAIMessage `json:"messages"`
	MaxTokens int             `json:"max_tokens"`
}

type openAIMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // string for system, []interface{} for user with images
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

// DescribeImage generates a description for an image.
// Parameters:
//   - ctx: context for cancellation and deadlines.
//   - imageData: raw image bytes (must be in a VLM-supported format: jpg, png).
//   - format: image format extension (jpg, png).
//
// Returns:
//   - string: generated description text.
//   - error: non-nil if the API request fails.
func (s *VLMService) DescribeImage(ctx context.Context, imageData []byte, format string) (string, error) {
	// Determine MIME type
	mimeType := getMIMEType(format)

	// Encode image to base64
	base64Image := base64.StdEncoding.EncodeToString(imageData)
	dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, base64Image)

	// Build request with system/user separation
	req := openAIRequest{
		Model: s.model,
		Messages: []openAIMessage{
			{
				Role:    "system",
				Content: prompts.VLMSystemPrompt,
			},
			{
				Role: "user",
				Content: []interface{}{
					openAITextContent{
						Type: "text",
						Text: prompts.VLMUserPrompt,
					},
					openAIImageContent{
						Type: "image_url",
						ImageURL: openAIImageURL{
							URL:    dataURL,
							Detail: "auto", // Use auto for better text recognition
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

// ExtractOCRText extracts text from an image using the VLM OCR prompt.
// Parameters:
//   - ctx: context for cancellation and deadlines.
//   - imageData: raw image bytes (must be in a VLM-supported format: jpg, png).
//   - format: image format extension (jpg, png).
//
// Returns:
//   - string: extracted OCR text (may be empty).
//   - error: non-nil if the API request fails.
func (s *VLMService) ExtractOCRText(ctx context.Context, imageData []byte, format string) (string, error) {
	// Determine MIME type
	mimeType := getMIMEType(format)

	// Encode image to base64
	base64Image := base64.StdEncoding.EncodeToString(imageData)
	dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, base64Image)

	req := openAIRequest{
		Model: s.model,
		Messages: []openAIMessage{
			{
				Role:    "system",
				Content: prompts.VLMOCRSystemPrompt,
			},
			{
				Role: "user",
				Content: []interface{}{
					openAITextContent{
						Type: "text",
						Text: prompts.VLMOCRUserPrompt,
					},
					openAIImageContent{
						Type: "image_url",
						ImageURL: openAIImageURL{
							URL:    dataURL,
							Detail: "auto",
						},
					},
				},
			},
		},
		MaxTokens: 400,
	}

	var resp openAIResponse
	httpResp, err := s.client.R().
		SetContext(ctx).
		SetBody(req).
		SetResult(&resp).
		Post(s.endpoint)

	if err != nil {
		return "", fmt.Errorf("failed to call VLM OCR API: %w", err)
	}

	if httpResp.StatusCode() < 200 || httpResp.StatusCode() >= 300 {
		errorMsg := fmt.Sprintf("HTTP %d", httpResp.StatusCode())
		if resp.Error != nil {
			errorMsg = fmt.Sprintf("HTTP %d: %s", httpResp.StatusCode(), resp.Error.Message)
		} else {
			errorMsg = fmt.Sprintf("HTTP %d: %s", httpResp.StatusCode(), string(httpResp.Body()))
		}
		return "", fmt.Errorf("VLM OCR API returned error: %s", errorMsg)
	}

	if resp.Error != nil {
		return "", fmt.Errorf("VLM OCR API error: %s", resp.Error.Message)
	}

	if len(resp.Choices) == 0 {
		errorMsg := fmt.Sprintf("no choices in response (status: %d)", httpResp.StatusCode())
		if len(httpResp.Body()) > 0 {
			errorMsg += fmt.Sprintf(", response body: %s", string(httpResp.Body()))
		}
		return "", fmt.Errorf("no response from VLM OCR API: %s", errorMsg)
	}

	return resp.Choices[0].Message.Content, nil
}

// DescribeImageFromURL generates a description for an image from URL.
// Parameters:
//   - ctx: context for cancellation and deadlines.
//   - imageURL: publicly accessible image URL.
//
// Returns:
//   - string: generated description text.
//   - error: non-nil if the API request fails.
func (s *VLMService) DescribeImageFromURL(ctx context.Context, imageURL string) (string, error) {
	// Build request with system/user separation
	req := openAIRequest{
		Model: s.model,
		Messages: []openAIMessage{
			{
				Role:    "system",
				Content: prompts.VLMSystemPrompt,
			},
			{
				Role: "user",
				Content: []interface{}{
					openAITextContent{
						Type: "text",
						Text: prompts.VLMUserPrompt,
					},
					openAIImageContent{
						Type: "image_url",
						ImageURL: openAIImageURL{
							URL:    imageURL,
							Detail: "auto", // Use auto for better text recognition
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
