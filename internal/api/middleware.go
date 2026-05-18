package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"

	"local-llm-gateway/internal/auth"
)

const requestIDHeader = "X-Request-ID"

type contextKey string

const requestIDContextKey contextKey = "request_id"
const apiKeyContextKey contextKey = "api_key"

func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get(requestIDHeader)
		if requestID == "" {
			requestID = generateRequestID()
		}

		w.Header().Set(requestIDHeader, requestID)

		ctx := context.WithValue(r.Context(), requestIDContextKey, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RequestIDFromContext(ctx context.Context) string {
	val, ok := ctx.Value(requestIDContextKey).(string)
	if !ok {
		return ""
	}

	return val
}

func APIKeyFromContext(ctx context.Context) *auth.APIKey {
	val, ok := ctx.Value(apiKeyContextKey).(*auth.APIKey)
	if !ok {
		return nil
	}
	return val
}

func APIKeyAuthMiddleware(authenticator *auth.Authenticator, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if shouldSkipAuth(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		if authenticator == nil {
			writeError(w, http.StatusInternalServerError, APIError{
				Message: "Authentication service unavailable.",
				Type:    "server_error",
				Code:    "auth_unavailable",
			})
			return
		}

		apiKey, err := authenticator.Authenticate(r.Context(), r.Header.Get("Authorization"))
		if err != nil {
			switch err {
			case auth.ErrMissingAPIKey, auth.ErrInvalidAPIKey:
				writeError(w, http.StatusUnauthorized, APIError{
					Message: "Invalid API key.",
					Type:    "invalid_request_error",
					Code:    "invalid_api_key",
				})
				return
			case auth.ErrDisabledAPIKey:
				writeError(w, http.StatusForbidden, APIError{
					Message: "API key is disabled.",
					Type:    "invalid_request_error",
					Code:    "api_key_disabled",
				})
				return
			default:
				writeError(w, http.StatusInternalServerError, APIError{
					Message: "Authentication service unavailable.",
					Type:    "server_error",
					Code:    "auth_unavailable",
				})
				return
			}
		}

		ctx := context.WithValue(r.Context(), apiKeyContextKey, apiKey)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func shouldSkipAuth(path string) bool {
	path = strings.TrimSpace(path)
	return path == "/health"
}

func generateRequestID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "req_fallback"
	}

	return "req_" + hex.EncodeToString(buf)
}
