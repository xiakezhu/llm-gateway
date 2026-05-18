package backend

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type LlamaCPPBackend struct {
	name    string
	baseURL string
	apiKey  string
	client  *http.Client
}

func NewLlamaCPPBackend(name, baseURL, apiKey string, timeout time.Duration) *LlamaCPPBackend {
	return &LlamaCPPBackend{
		name:    name,
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  strings.TrimSpace(apiKey),
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

func (b *LlamaCPPBackend) Name() string {
	return b.name
}

func (b *LlamaCPPBackend) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	upstreamReq := llamaChatCompletionRequest{
		Model:       req.Model,
		Messages:    make([]llamaChatMessage, 0, len(req.Messages)),
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Stream:      false,
	}

	for _, msg := range req.Messages {
		upstreamReq.Messages = append(upstreamReq.Messages, llamaChatMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	body, err := json.Marshal(upstreamReq)
	if err != nil {
		return nil, fmt.Errorf("%w: marshal request: %v", ErrBackendUnavailable, err)
	}

	url := b.baseURL + "/v1/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("%w: create request: %v", ErrBackendUnavailable, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if b.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+b.apiKey)
	}

	resp, err := b.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%w: request failed: %v", ErrBackendUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		rawBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("%w: status=%d body=%s", ErrBackendUnavailable, resp.StatusCode, string(rawBody))
	}

	var upstreamResp llamaChatCompletionResponse
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

func (b *LlamaCPPBackend) ChatStream(ctx context.Context, req ChatRequest) (<-chan ChatChunk, error) {
	upstreamReq := llamaChatCompletionRequest{
		Model:       req.Model,
		Messages:    make([]llamaChatMessage, 0, len(req.Messages)),
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Stream:      true,
	}

	for _, msg := range req.Messages {
		upstreamReq.Messages = append(upstreamReq.Messages, llamaChatMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	body, err := json.Marshal(upstreamReq)
	if err != nil {
		return nil, fmt.Errorf("%w: marshal request: %v", ErrBackendUnavailable, err)
	}

	url := b.baseURL + "/v1/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("%w: create request: %v", ErrBackendUnavailable, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if b.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+b.apiKey)
	}

	resp, err := b.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%w: request failed: %v", ErrBackendUnavailable, err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		defer resp.Body.Close()
		rawBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("%w: status=%d body=%s", ErrBackendUnavailable, resp.StatusCode, string(rawBody))
	}

	ch := make(chan ChatChunk)
	go func() {
		defer close(ch)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, ":") {
				continue
			}
			if !strings.HasPrefix(line, "data:") {
				continue
			}

			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "[DONE]" {
				return
			}

			var upstreamChunk llamaChatCompletionStreamChunk
			if err := json.Unmarshal([]byte(data), &upstreamChunk); err != nil {
				ch <- ChatChunk{Err: fmt.Errorf("%w: decode stream chunk: %v", ErrBackendUnavailable, err)}
				return
			}
			if len(upstreamChunk.Choices) == 0 {
				continue
			}

			choice := upstreamChunk.Choices[0]
			ch <- ChatChunk{
				Content:      choice.Delta.Content,
				FinishReason: choice.FinishReason,
			}
		}

		if err := scanner.Err(); err != nil {
			ch <- ChatChunk{Err: fmt.Errorf("%w: read stream response: %v", ErrBackendUnavailable, err)}
		}
	}()

	return ch, nil
}

func (b *LlamaCPPBackend) Health(ctx context.Context) error {
	url := b.baseURL + "/health"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("%w: create health request: %v", ErrBackendUnavailable, err)
	}
	if b.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+b.apiKey)
	}

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

type llamaChatCompletionRequest struct {
	Model       string             `json:"model"`
	Messages    []llamaChatMessage `json:"messages"`
	Temperature *float64           `json:"temperature,omitempty"`
	MaxTokens   *int               `json:"max_tokens,omitempty"`
	Stream      bool               `json:"stream,omitempty"`
}

type llamaChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type llamaChatCompletionResponse struct {
	Model   string               `json:"model"`
	Choices []llamaChatChoice    `json:"choices"`
	Usage   llamaChatUsageTokens `json:"usage"`
}

type llamaChatChoice struct {
	Message      llamaChatMessage `json:"message"`
	FinishReason string           `json:"finish_reason"`
}

type llamaChatUsageTokens struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type llamaChatCompletionStreamChunk struct {
	Choices []llamaChatStreamChoice `json:"choices"`
}

type llamaChatStreamChoice struct {
	Delta        llamaChatMessage `json:"delta"`
	FinishReason string           `json:"finish_reason"`
}
