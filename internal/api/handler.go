package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"local-llm-gateway/internal/backend"
	"local-llm-gateway/internal/router"
)

type modelResolver interface {
	Resolve(model string) (backend.Backend, error)
}

type modelLister interface {
	Models() []string
}

type Handler struct {
	resolver modelResolver
}

func NewHandler(resolver modelResolver) *Handler {
	return &Handler{
		resolver: resolver,
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/health", h.handleHealth)
	mux.HandleFunc("/v1/models", h.handleModels)
	mux.HandleFunc("/v1/chat/completions", h.handleChatCompletions)
	mux.HandleFunc("/metrics", h.handleMetrics)
}

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowedError(w)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) handleModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowedError(w)
		return
	}

	modelIDs := []string{}
	if lister, ok := h.resolver.(modelLister); ok {
		modelIDs = append(modelIDs, lister.Models()...)
		sort.Strings(modelIDs)
	}

	data := make([]map[string]any, 0, len(modelIDs))
	for _, modelID := range modelIDs {
		data = append(data, map[string]any{
			"id":       modelID,
			"object":   "model",
			"created":  0,
			"owned_by": "local-llm-gateway",
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"object": "list",
		"data":   data,
	})
}

func (h *Handler) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowedError(w)
		return
	}

	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, "# HELP llm_gateway_info Gateway build information.\n")
	_, _ = io.WriteString(w, "# TYPE llm_gateway_info gauge\n")
	_, _ = io.WriteString(w, "llm_gateway_info 1\n")
}

func (h *Handler) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowedError(w)
		return
	}

	defer r.Body.Close()

	var req ChatCompletionRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil {
		writeInvalidRequestError(w, "Invalid JSON request body.")
		return
	}

	if err := validateChatCompletionRequest(req); err != nil {
		writeInvalidRequestError(w, err.Error())
		return
	}

	if req.Stream {
		h.handleChatCompletionsStream(w, r, req)
		return
	}

	if h.resolver == nil {
		writeError(w, http.StatusInternalServerError, APIError{
			Message: "Model router is not initialized.",
			Type:    "server_error",
			Code:    "backend_resolution_failed",
		})
		return
	}

	selectedBackend, err := h.resolver.Resolve(req.Model)
	if err != nil {
		if errors.Is(err, router.ErrModelNotFound) {
			writeError(w, http.StatusBadRequest, APIError{
				Message: "The requested model is not configured in the gateway.",
				Type:    "invalid_request_error",
				Code:    "model_not_found",
			})
			return
		}

		writeError(w, http.StatusInternalServerError, APIError{
			Message: "Failed to resolve model backend.",
			Type:    "server_error",
			Code:    "backend_resolution_failed",
		})
		return
	}

	backendReq := toBackendChatRequest(req)
	backendResp, err := selectedBackend.Chat(r.Context(), backendReq)
	if err != nil {
		if errors.Is(err, backend.ErrBackendUnavailable) {
			writeError(w, http.StatusBadGateway, APIError{
				Message: "The selected model backend is temporarily unavailable.",
				Type:    "backend_unavailable",
				Code:    "backend_unavailable",
			})
			return
		}

		writeError(w, http.StatusInternalServerError, APIError{
			Message: "Unexpected backend error.",
			Type:    "server_error",
			Code:    "backend_error",
		})
		return
	}

	finishReason := backendResp.FinishReason
	if finishReason == "" {
		finishReason = "stop"
	}

	resp := ChatCompletionResponse{
		ID:      buildChatCompletionID(r.Context()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   backendResp.Model,
		Choices: []ChatCompletionChoice{
			{
				Index: 0,
				Message: ChatMessage{
					Role:    "assistant",
					Content: backendResp.Content,
				},
				FinishReason: finishReason,
			},
		},
		Usage: ChatCompletionUsage{
			PromptTokens:     backendResp.Usage.PromptTokens,
			CompletionTokens: backendResp.Usage.CompletionTokens,
			TotalTokens:      backendResp.Usage.TotalTokens,
		},
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) handleChatCompletionsStream(w http.ResponseWriter, r *http.Request, req ChatCompletionRequest) {
	if h.resolver == nil {
		writeError(w, http.StatusInternalServerError, APIError{
			Message: "Model router is not initialized.",
			Type:    "server_error",
			Code:    "backend_resolution_failed",
		})
		return
	}

	selectedBackend, err := h.resolver.Resolve(req.Model)
	if err != nil {
		if errors.Is(err, router.ErrModelNotFound) {
			writeError(w, http.StatusBadRequest, APIError{
				Message: "The requested model is not configured in the gateway.",
				Type:    "invalid_request_error",
				Code:    "model_not_found",
			})
			return
		}

		writeError(w, http.StatusInternalServerError, APIError{
			Message: "Failed to resolve model backend.",
			Type:    "server_error",
			Code:    "backend_resolution_failed",
		})
		return
	}

	backendReq := toBackendChatRequest(req)
	streamCh, err := selectedBackend.ChatStream(r.Context(), backendReq)
	if err != nil {
		if errors.Is(err, backend.ErrStreamNotSupported) {
			writeError(w, http.StatusNotImplemented, APIError{
				Message: "Streaming is not supported by selected backend.",
				Type:    "invalid_request_error",
				Code:    "stream_not_supported",
			})
			return
		}
		if errors.Is(err, backend.ErrBackendUnavailable) {
			writeError(w, http.StatusBadGateway, APIError{
				Message: "The selected model backend is temporarily unavailable.",
				Type:    "backend_unavailable",
				Code:    "backend_unavailable",
			})
			return
		}
		writeError(w, http.StatusInternalServerError, APIError{
			Message: "Unexpected backend streaming error.",
			Type:    "server_error",
			Code:    "backend_error",
		})
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, APIError{
			Message: "Streaming is not supported by server.",
			Type:    "server_error",
			Code:    "streaming_not_supported",
		})
		return
	}

	firstChunk, hasChunk := <-streamCh
	if !hasChunk {
		writeError(w, http.StatusBadGateway, APIError{
			Message: "The selected model backend returned an empty stream.",
			Type:    "backend_unavailable",
			Code:    "backend_unavailable",
		})
		return
	}
	if firstChunk.Err != nil {
		writeError(w, http.StatusBadGateway, APIError{
			Message: "The selected model backend is temporarily unavailable.",
			Type:    "backend_unavailable",
			Code:    "backend_unavailable",
		})
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	chatID := buildChatCompletionID(r.Context())
	created := time.Now().Unix()

	if err := writeSSEChunk(w, flusher, ChatCompletionChunkResponse{
		ID:      chatID,
		Object:  "chat.completion.chunk",
		Created: created,
		Model:   req.Model,
		Choices: []ChatCompletionChunkChoice{
			{
				Index: 0,
				Delta: ChatCompletionChunkDelta{
					Role: "assistant",
				},
				FinishReason: nil,
			},
		},
	}); err != nil {
		return
	}

	writeFromBackendChunk := func(chunk backend.ChatChunk) error {
		if chunk.Err != nil {
			return chunk.Err
		}

		var finishReason *string
		if chunk.FinishReason != "" {
			reason := chunk.FinishReason
			finishReason = &reason
		}

		return writeSSEChunk(w, flusher, ChatCompletionChunkResponse{
			ID:      chatID,
			Object:  "chat.completion.chunk",
			Created: created,
			Model:   req.Model,
			Choices: []ChatCompletionChunkChoice{
				{
					Index: 0,
					Delta: ChatCompletionChunkDelta{
						Content: chunk.Content,
					},
					FinishReason: finishReason,
				},
			},
		})
	}

	if err := writeFromBackendChunk(firstChunk); err != nil {
		return
	}

	var finalReason string
	if firstChunk.FinishReason != "" {
		finalReason = firstChunk.FinishReason
	}

	for chunk := range streamCh {
		if err := writeFromBackendChunk(chunk); err != nil {
			return
		}
		if chunk.FinishReason != "" {
			finalReason = chunk.FinishReason
		}
	}

	if finalReason == "" {
		finalReason = "stop"
		if err := writeSSEChunk(w, flusher, ChatCompletionChunkResponse{
			ID:      chatID,
			Object:  "chat.completion.chunk",
			Created: created,
			Model:   req.Model,
			Choices: []ChatCompletionChunkChoice{
				{
					Index:        0,
					Delta:        ChatCompletionChunkDelta{},
					FinishReason: &finalReason,
				},
			},
		}); err != nil {
			return
		}
	}

	_, _ = io.WriteString(w, "data: [DONE]\n\n")
	flusher.Flush()
}

func writeSSEChunk(w http.ResponseWriter, flusher http.Flusher, payload ChatCompletionChunkResponse) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	if _, err := io.WriteString(w, "data: "+string(raw)+"\n\n"); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}

func toBackendChatRequest(req ChatCompletionRequest) backend.ChatRequest {
	messages := make([]backend.ChatMessage, 0, len(req.Messages))
	for _, msg := range req.Messages {
		messages = append(messages, backend.ChatMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	return backend.ChatRequest{
		Model:       req.Model,
		Messages:    messages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Stream:      req.Stream,
	}
}

func validateChatCompletionRequest(req ChatCompletionRequest) error {
	if strings.TrimSpace(req.Model) == "" {
		return fmt.Errorf("Field 'model' is required")
	}

	if len(req.Messages) == 0 {
		return fmt.Errorf("Field 'messages' must contain at least one message")
	}

	for i, msg := range req.Messages {
		if strings.TrimSpace(msg.Role) == "" {
			return fmt.Errorf("messages[%d].role is required", i)
		}
		if strings.TrimSpace(msg.Content) == "" {
			return fmt.Errorf("messages[%d].content is required", i)
		}
	}

	return nil
}

func buildChatCompletionID(ctx context.Context) string {
	requestID := RequestIDFromContext(ctx)
	if requestID != "" {
		return "chatcmpl_" + requestID
	}

	return "chatcmpl_" + fmt.Sprintf("%d", time.Now().UnixNano())
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}
