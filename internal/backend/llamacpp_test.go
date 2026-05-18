package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestLlamaCPPBackendChat(t *testing.T) {
	var gotPath string
	var gotModel string
	var gotAuth string

	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		defer r.Body.Close()

		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		gotModel, _ = req["model"].(string)

		respBody, _ := json.Marshal(map[string]any{
			"model": "local-llama",
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"role":    "assistant",
						"content": "backend reply",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     10,
				"completion_tokens": 12,
				"total_tokens":      22,
			},
		})
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewReader(respBody)),
		}, nil
	})

	b := NewLlamaCPPBackend("llama.cpp", "http://llama.local", "llama-provider-key", 3*time.Second)
	b.client = &http.Client{Transport: rt}
	resp, err := b.Chat(context.Background(), ChatRequest{
		Model: "local-llama",
		Messages: []ChatMessage{
			{Role: "user", Content: "hello"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected chat error: %v", err)
	}

	if gotPath != "/v1/chat/completions" {
		t.Fatalf("expected request path /v1/chat/completions, got %s", gotPath)
	}
	if gotModel != "local-llama" {
		t.Fatalf("expected forwarded model local-llama, got %s", gotModel)
	}
	if gotAuth != "Bearer llama-provider-key" {
		t.Fatalf("expected auth header set, got %s", gotAuth)
	}
	if resp.Content != "backend reply" {
		t.Fatalf("expected backend reply, got %s", resp.Content)
	}
	if resp.Usage.TotalTokens != 22 {
		t.Fatalf("expected total tokens 22, got %d", resp.Usage.TotalTokens)
	}
}

func TestLlamaCPPBackendChatUpstreamError(t *testing.T) {
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusServiceUnavailable,
			Header:     http.Header{"Content-Type": []string{"text/plain"}},
			Body:       io.NopCloser(strings.NewReader("upstream unavailable")),
		}, nil
	})

	b := NewLlamaCPPBackend("llama.cpp", "http://llama.local", "", 3*time.Second)
	b.client = &http.Client{Transport: rt}
	_, err := b.Chat(context.Background(), ChatRequest{
		Model: "local-llama",
		Messages: []ChatMessage{
			{Role: "user", Content: "hello"},
		},
	})

	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, ErrBackendUnavailable) {
		t.Fatalf("expected ErrBackendUnavailable, got %v", err)
	}
}

func TestLlamaCPPBackendChatStream(t *testing.T) {
	var gotPath string
	var gotAuth string

	sseBody := strings.Join([]string{
		`data: {"choices":[{"delta":{"content":"Hello"},"finish_reason":""}]}`,
		"",
		`data: {"choices":[{"delta":{"content":" world"},"finish_reason":"stop"}]}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")

	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
			Body:       io.NopCloser(strings.NewReader(sseBody)),
		}, nil
	})

	b := NewLlamaCPPBackend("llama.cpp", "http://llama.local", "llama-provider-key", 3*time.Second)
	b.client = &http.Client{Transport: rt}

	ch, err := b.ChatStream(context.Background(), ChatRequest{
		Model: "local-llama",
		Messages: []ChatMessage{
			{Role: "user", Content: "hello"},
		},
		Stream: true,
	})
	if err != nil {
		t.Fatalf("unexpected stream error: %v", err)
	}

	var chunks []ChatChunk
	for chunk := range ch {
		chunks = append(chunks, chunk)
	}

	if gotPath != "/v1/chat/completions" {
		t.Fatalf("expected request path /v1/chat/completions, got %s", gotPath)
	}
	if gotAuth != "Bearer llama-provider-key" {
		t.Fatalf("expected auth header set, got %s", gotAuth)
	}
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
	if chunks[0].Content != "Hello" || chunks[1].Content != " world" {
		t.Fatalf("unexpected streamed content: %+v", chunks)
	}
	if chunks[1].FinishReason != "stop" {
		t.Fatalf("expected finish reason stop, got %s", chunks[1].FinishReason)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
