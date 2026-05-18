package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type OpenAIBackend struct {
	name    string
	baseURL string
	apiKey  string
	client  *http.Client
}

func NewOpenAIBackend(name, baseURL, apiKey string, timeout time.Duration) *OpenAIBackend {
	return &OpenAIBackend{
		name:    name,
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  strings.TrimSpace(apiKey),
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

func (b *OpenAIBackend) Name() string {
	return b.name
}

func (b *OpenAIBackend) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	if b.apiKey == "" {
		return nil, fmt.Errorf("%w: openai backend api key is empty", ErrBackendUnavailable)
	}

	upstreamReq := openAIChatCompletionRequest{
		Model:       req.Model,
		Messages:    make([]openAIChatMessage, 0, len(req.Messages)),
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Stream:      false,
	}

	for _, msg := range req.Messages {
		upstreamReq.Messages = append(upstreamReq.Messages, openAIChatMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	body, err := json.Marshal(upstreamReq)
	if err != nil {
		return nil, fmt.Errorf("%w: marshal request: %v", ErrBackendUnavailable, err)
	}

	url := b.baseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("%w: create request: %v", ErrBackendUnavailable, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+b.apiKey)

	resp, err := b.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%w: request failed: %v", ErrBackendUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		rawBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("%w: status=%d body=%s", ErrBackendUnavailable, resp.StatusCode, string(rawBody))
	}

	var upstreamResp openAIChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&upstreamResp); err != nil {
		return nil, fmt.Errorf("%w: decode response: %v", ErrBackendUnavailable, err)
	}

	if len(upstreamResp.Choices) == 0 {
		return nil, fmt.Errorf("%w: empty choices", ErrBackendUnavailable)
	}

	content := upstreamResp.Choices[0].Message.Content
	finishReason := upstreamResp.Choices[0].FinishReason
	if finishReason == "" {
		finishReason = "stop"
	}

	model := upstreamResp.Model
	if model == "" {
		model = req.Model
	}

	return &ChatResponse{
		Model:        model,
		Content:      content,
		FinishReason: finishReason,
		Usage: Usage{
			PromptTokens:     upstreamResp.Usage.PromptTokens,
			CompletionTokens: upstreamResp.Usage.CompletionTokens,
			TotalTokens:      upstreamResp.Usage.TotalTokens,
		},
	}, nil
}

func (b *OpenAIBackend) ChatStream(ctx context.Context, req ChatRequest) (<-chan ChatChunk, error) {
	return nil, ErrStreamNotSupported
}

func (b *OpenAIBackend) Health(ctx context.Context) error {
	if b.apiKey == "" {
		return fmt.Errorf("%w: openai backend api key is empty", ErrBackendUnavailable)
	}

	url := b.baseURL + "/models"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("%w: create health request: %v", ErrBackendUnavailable, err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+b.apiKey)

	resp, err := b.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("%w: health request failed: %v", ErrBackendUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("%w: health status=%d", ErrBackendUnavailable, resp.StatusCode)
	}

	return nil
}

type openAIChatCompletionRequest struct {
	Model       string              `json:"model"`
	Messages    []openAIChatMessage `json:"messages"`
	Temperature *float64            `json:"temperature,omitempty"`
	MaxTokens   *int                `json:"max_tokens,omitempty"`
	Stream      bool                `json:"stream,omitempty"`
}

type openAIChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIChatCompletionResponse struct {
	Model   string                `json:"model"`
	Choices []openAIChatChoice    `json:"choices"`
	Usage   openAIChatUsageTokens `json:"usage"`
}

type openAIChatChoice struct {
	Message      openAIChatMessage `json:"message"`
	FinishReason string            `json:"finish_reason"`
}

type openAIChatUsageTokens struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}
