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

func TestOpenAIBackendChatAddsAuthHeader(t *testing.T) {
	var gotAuth string
	var gotPath string

	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")

		respBody, _ := json.Marshal(map[string]any{
			"model": "gpt-4o-mini",
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"role":    "assistant",
						"content": "hello from openai backend",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     4,
				"completion_tokens": 7,
				"total_tokens":      11,
			},
		})
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewReader(respBody)),
		}, nil
	})

	b := NewOpenAIBackend("openai", "https://api.openai.com/v1", "sk-provider", 3*time.Second)
	b.client = &http.Client{Transport: rt}

	resp, err := b.Chat(context.Background(), ChatRequest{
		Model: "gpt-4o-mini",
		Messages: []ChatMessage{
			{Role: "user", Content: "hello"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected chat error: %v", err)
	}

	if gotPath != "/v1/chat/completions" {
		t.Fatalf("expected path /v1/chat/completions, got %s", gotPath)
	}
	if gotAuth != "Bearer sk-provider" {
		t.Fatalf("expected auth header set, got %s", gotAuth)
	}
	if resp.Content != "hello from openai backend" {
		t.Fatalf("expected content from backend, got %s", resp.Content)
	}
}

func TestOpenAIBackendChatMissingAPIKey(t *testing.T) {
	b := NewOpenAIBackend("openai", "https://api.openai.com/v1", "", 3*time.Second)
	_, err := b.Chat(context.Background(), ChatRequest{
		Model: "gpt-4o-mini",
		Messages: []ChatMessage{
			{Role: "user", Content: "hello"},
		},
	})
	if !errors.Is(err, ErrBackendUnavailable) {
		t.Fatalf("expected ErrBackendUnavailable, got %v", err)
	}
}

func TestOpenAIBackendChatUpstreamError(t *testing.T) {
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusUnauthorized,
			Header:     http.Header{"Content-Type": []string{"text/plain"}},
			Body:       io.NopCloser(strings.NewReader("unauthorized")),
		}, nil
	})

	b := NewOpenAIBackend("openai", "https://api.openai.com/v1", "sk-provider", 3*time.Second)
	b.client = &http.Client{Transport: rt}

	_, err := b.Chat(context.Background(), ChatRequest{
		Model: "gpt-4o-mini",
		Messages: []ChatMessage{
			{Role: "user", Content: "hello"},
		},
	})
	if !errors.Is(err, ErrBackendUnavailable) {
		t.Fatalf("expected ErrBackendUnavailable, got %v", err)
	}
}
