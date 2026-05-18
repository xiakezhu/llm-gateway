package backend

import "context"

type Backend interface {
	Name() string
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
	ChatStream(ctx context.Context, req ChatRequest) (<-chan ChatChunk, error)
	Health(ctx context.Context) error
}

type ChatRequest struct {
	Model       string
	Messages    []ChatMessage
	Temperature *float64
	MaxTokens   *int
	Stream      bool
}

type ChatMessage struct {
	Role    string
	Content string
}

type ChatResponse struct {
	Model        string
	Content      string
	FinishReason string
	Usage        Usage
}

type ChatChunk struct {
	Content      string
	FinishReason string
	Err          error
}

type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}
