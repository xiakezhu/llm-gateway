package router

import (
	"context"
	"errors"
	"testing"

	"local-llm-gateway/internal/backend"
)

type dummyBackend struct{}

func (b dummyBackend) Name() string { return "dummy" }

func (b dummyBackend) Chat(ctx context.Context, req backend.ChatRequest) (*backend.ChatResponse, error) {
	return &backend.ChatResponse{}, nil
}

func (b dummyBackend) ChatStream(ctx context.Context, req backend.ChatRequest) (<-chan backend.ChatChunk, error) {
	return nil, nil
}

func (b dummyBackend) Health(ctx context.Context) error { return nil }

func TestModelRouterResolve(t *testing.T) {
	r, err := NewModelRouter(map[string]backend.Backend{
		"local-llama": dummyBackend{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	b, err := r.Resolve("local-llama")
	if err != nil {
		t.Fatalf("unexpected resolve error: %v", err)
	}
	if b.Name() != "dummy" {
		t.Fatalf("expected dummy backend, got %s", b.Name())
	}
}

func TestModelRouterModelNotFound(t *testing.T) {
	r, err := NewModelRouter(map[string]backend.Backend{
		"local-llama": dummyBackend{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = r.Resolve("unknown")
	if !errors.Is(err, ErrModelNotFound) {
		t.Fatalf("expected ErrModelNotFound, got %v", err)
	}
}
