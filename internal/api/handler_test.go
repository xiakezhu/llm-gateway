package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"local-llm-gateway/internal/backend"
	"local-llm-gateway/internal/router"
)

type fakeResolver struct {
	resolveFn func(model string) (backend.Backend, error)
}

func (r fakeResolver) Resolve(model string) (backend.Backend, error) {
	return r.resolveFn(model)
}

type fakeBackend struct {
	chatFn       func(ctx context.Context, req backend.ChatRequest) (*backend.ChatResponse, error)
	chatStreamFn func(ctx context.Context, req backend.ChatRequest) (<-chan backend.ChatChunk, error)
}

func (b fakeBackend) Name() string {
	return "fake"
}

func (b fakeBackend) Chat(ctx context.Context, req backend.ChatRequest) (*backend.ChatResponse, error) {
	return b.chatFn(ctx, req)
}

func (b fakeBackend) ChatStream(ctx context.Context, req backend.ChatRequest) (<-chan backend.ChatChunk, error) {
	if b.chatStreamFn == nil {
		return nil, backend.ErrStreamNotSupported
	}
	return b.chatStreamFn(ctx, req)
}

func (b fakeBackend) Health(ctx context.Context) error {
	return nil
}

func newTestServer(resolver modelResolver) http.Handler {
	mux := http.NewServeMux()
	NewHandler(resolver).RegisterRoutes(mux)
	return RequestIDMiddleware(mux)
}

func TestHandleHealth(t *testing.T) {
	server := newTestServer(fakeResolver{
		resolveFn: func(model string) (backend.Backend, error) {
			return nil, errors.New("not used")
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
}

func TestHandleChatCompletionsSuccess(t *testing.T) {
	server := newTestServer(fakeResolver{
		resolveFn: func(model string) (backend.Backend, error) {
			return fakeBackend{
				chatFn: func(ctx context.Context, req backend.ChatRequest) (*backend.ChatResponse, error) {
					if req.Model != "local-llama" {
						t.Fatalf("expected model local-llama, got %s", req.Model)
					}
					return &backend.ChatResponse{
						Model:        req.Model,
						Content:      "hello from backend",
						FinishReason: "stop",
						Usage: backend.Usage{
							PromptTokens:     3,
							CompletionTokens: 5,
							TotalTokens:      8,
						},
					}, nil
				},
			}, nil
		},
	})

	payload := ChatCompletionRequest{
		Model: "local-llama",
		Messages: []ChatMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp ChatCompletionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Object != "chat.completion" {
		t.Fatalf("expected object chat.completion, got %s", resp.Object)
	}
	if resp.Model != payload.Model {
		t.Fatalf("expected model %s, got %s", payload.Model, resp.Model)
	}
	if len(resp.Choices) != 1 {
		t.Fatalf("expected 1 choice, got %d", len(resp.Choices))
	}
	if got := resp.Choices[0].Message.Content; got != "hello from backend" {
		t.Fatalf("expected backend content, got %s", got)
	}
	if resp.Usage.TotalTokens != 8 {
		t.Fatalf("expected usage total tokens 8, got %d", resp.Usage.TotalTokens)
	}
}

func TestHandleChatCompletionsValidation(t *testing.T) {
	server := newTestServer(fakeResolver{
		resolveFn: func(model string) (backend.Backend, error) {
			return nil, errors.New("not used")
		},
	})

	payload := `{"messages":[{"role":"user","content":"hello"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(payload))
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestHandleChatCompletionsStreamNotSupported(t *testing.T) {
	server := newTestServer(fakeResolver{
		resolveFn: func(model string) (backend.Backend, error) {
			return fakeBackend{
				chatFn: func(ctx context.Context, req backend.ChatRequest) (*backend.ChatResponse, error) {
					return nil, errors.New("not used")
				},
				chatStreamFn: func(ctx context.Context, req backend.ChatRequest) (<-chan backend.ChatChunk, error) {
					return nil, backend.ErrStreamNotSupported
				},
			}, nil
		},
	})

	payload := ChatCompletionRequest{
		Model: "local-llama",
		Messages: []ChatMessage{
			{Role: "user", Content: "Hello"},
		},
		Stream: true,
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("expected status 501, got %d", rec.Code)
	}
}

func TestHandleChatCompletionsStreamSuccess(t *testing.T) {
	server := newTestServer(fakeResolver{
		resolveFn: func(model string) (backend.Backend, error) {
			return fakeBackend{
				chatFn: func(ctx context.Context, req backend.ChatRequest) (*backend.ChatResponse, error) {
					return nil, errors.New("not used")
				},
				chatStreamFn: func(ctx context.Context, req backend.ChatRequest) (<-chan backend.ChatChunk, error) {
					ch := make(chan backend.ChatChunk, 2)
					ch <- backend.ChatChunk{Content: "Hello"}
					ch <- backend.ChatChunk{Content: " world", FinishReason: "stop"}
					close(ch)
					return ch, nil
				},
			}, nil
		},
	})

	payload := ChatCompletionRequest{
		Model: "local-llama",
		Messages: []ChatMessage{
			{Role: "user", Content: "Hello"},
		},
		Stream: true,
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("expected content-type text/event-stream, got %s", got)
	}

	respText := rec.Body.String()
	if !strings.Contains(respText, "\"object\":\"chat.completion.chunk\"") {
		t.Fatalf("expected stream chunk object in response body, got %s", respText)
	}
	if !strings.Contains(respText, "\"content\":\"Hello\"") {
		t.Fatalf("expected streamed content chunk, got %s", respText)
	}
	if !strings.Contains(respText, "data: [DONE]") {
		t.Fatalf("expected done event, got %s", respText)
	}
}

func TestHandleChatCompletionsModelNotFound(t *testing.T) {
	server := newTestServer(fakeResolver{
		resolveFn: func(model string) (backend.Backend, error) {
			return nil, router.ErrModelNotFound
		},
	})

	payload := ChatCompletionRequest{
		Model: "missing-model",
		Messages: []ChatMessage{
			{Role: "user", Content: "Hello"},
		},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestHandleChatCompletionsBackendUnavailable(t *testing.T) {
	server := newTestServer(fakeResolver{
		resolveFn: func(model string) (backend.Backend, error) {
			return fakeBackend{
				chatFn: func(ctx context.Context, req backend.ChatRequest) (*backend.ChatResponse, error) {
					return nil, backend.ErrBackendUnavailable
				},
			}, nil
		},
	})

	payload := ChatCompletionRequest{
		Model: "local-llama",
		Messages: []ChatMessage{
			{Role: "user", Content: "Hello"},
		},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected status 502, got %d", rec.Code)
	}
}
