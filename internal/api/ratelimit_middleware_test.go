package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"local-llm-gateway/internal/auth"
	"local-llm-gateway/internal/ratelimit"
)

func TestRateLimitMiddlewareAllowsWithinLimit(t *testing.T) {
	manager := ratelimit.NewManager()
	handler := RateLimitMiddleware(manager, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req = withTestAPIKey(req, &auth.APIKey{ID: "key_1", RPMLimit: 1})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestRateLimitMiddlewareRejectsOverLimit(t *testing.T) {
	manager := ratelimit.NewManager()
	handler := RateLimitMiddleware(manager, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req1 := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req1 = withTestAPIKey(req1, &auth.APIKey{ID: "key_1", RPMLimit: 1})
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("expected first request 200, got %d", rec1.Code)
	}

	req2 := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req2 = withTestAPIKey(req2, &auth.APIKey{ID: "key_1", RPMLimit: 1})
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected second request 429, got %d", rec2.Code)
	}
	if got := rec2.Header().Get("Retry-After"); got != "60" {
		t.Fatalf("expected Retry-After 60, got %s", got)
	}
}

func TestRateLimitMiddlewareRetryAfterUsesRPMLimit(t *testing.T) {
	manager := ratelimit.NewManager()
	handler := RateLimitMiddleware(manager, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		req = withTestAPIKey(req, &auth.APIKey{ID: "key_1", RPMLimit: 2})
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected request %d status 200, got %d", i+1, rec.Code)
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req = withTestAPIKey(req, &auth.APIKey{ID: "key_1", RPMLimit: 2})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected third request 429, got %d", rec.Code)
	}
	if got := rec.Header().Get("Retry-After"); got != "30" {
		t.Fatalf("expected Retry-After 30, got %s", got)
	}
}

func TestRateLimitMiddlewareRejectsInvalidLimitAsServerError(t *testing.T) {
	manager := ratelimit.NewManager()
	handler := RateLimitMiddleware(manager, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req = withTestAPIKey(req, &auth.APIKey{ID: "key_1", RPMLimit: 0})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	if got := rec.Header().Get("Retry-After"); got != "" {
		t.Fatalf("expected no Retry-After header, got %s", got)
	}

	var body APIErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if body.Error.Code != "rate_limit_invalid_config" {
		t.Fatalf("expected rate_limit_invalid_config, got %s", body.Error.Code)
	}
}

func TestRateLimitMiddlewareRequiresAPIKeyContext(t *testing.T) {
	manager := ratelimit.NewManager()
	handler := RateLimitMiddleware(manager, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestRateLimitMiddlewareSkipsHealth(t *testing.T) {
	handler := RateLimitMiddleware(nil, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func withTestAPIKey(req *http.Request, apiKey *auth.APIKey) *http.Request {
	ctx := context.WithValue(req.Context(), apiKeyContextKey, apiKey)
	return req.WithContext(ctx)
}
