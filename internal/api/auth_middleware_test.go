package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"local-llm-gateway/internal/auth"
)

func TestAPIKeyAuthMiddlewareMissingKey(t *testing.T) {
	authenticator := newTestAuthenticator(t)
	handler := APIKeyAuthMiddleware(authenticator, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestAPIKeyAuthMiddlewareInvalidKey(t *testing.T) {
	authenticator := newTestAuthenticator(t)
	handler := APIKeyAuthMiddleware(authenticator, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer sk-invalid")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestAPIKeyAuthMiddlewareDisabledKey(t *testing.T) {
	authenticator := newTestAuthenticator(t)
	handler := APIKeyAuthMiddleware(authenticator, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer sk-disabled")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestAPIKeyAuthMiddlewareSuccess(t *testing.T) {
	authenticator := newTestAuthenticator(t)
	handler := APIKeyAuthMiddleware(authenticator, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := APIKeyFromContext(r.Context())
		if apiKey == nil {
			t.Fatalf("expected api key in context")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer sk-valid")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestAPIKeyAuthMiddlewareSkipsHealth(t *testing.T) {
	handler := APIKeyAuthMiddleware(nil, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func newTestAuthenticator(t *testing.T) *auth.Authenticator {
	t.Helper()

	repo, err := auth.NewInMemoryRepository([]auth.APIKey{
		{
			ID:        "1",
			Name:      "valid",
			KeyPrefix: auth.KeyPrefix("sk-valid"),
			KeyHash:   auth.HashAPIKey("sk-valid"),
			Status:    auth.APIKeyStatusActive,
		},
		{
			ID:        "2",
			Name:      "disabled",
			KeyPrefix: auth.KeyPrefix("sk-disabled"),
			KeyHash:   auth.HashAPIKey("sk-disabled"),
			Status:    auth.APIKeyStatusDisabled,
		},
	})
	if err != nil {
		t.Fatalf("new auth repo: %v", err)
	}

	return auth.NewAuthenticator(repo)
}
