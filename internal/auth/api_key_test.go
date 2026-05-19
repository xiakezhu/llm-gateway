package auth

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestAuthenticatorSuccess(t *testing.T) {
	repo, err := NewInMemoryRepository([]APIKey{
		{
			ID:        "1",
			Name:      "demo",
			KeyPrefix: KeyPrefix("sk-valid"),
			KeyHash:   HashAPIKey("sk-valid"),
			Status:    APIKeyStatusActive,
		},
	})
	if err != nil {
		t.Fatalf("new repo: %v", err)
	}

	authenticator := NewAuthenticator(repo)
	apiKey, err := authenticator.Authenticate(context.Background(), "Bearer sk-valid")
	if err != nil {
		t.Fatalf("authenticate error: %v", err)
	}
	if apiKey.Name != "demo" {
		t.Fatalf("expected key name demo, got %s", apiKey.Name)
	}
}

func TestAuthenticatorMissingKey(t *testing.T) {
	repo, _ := NewInMemoryRepository([]APIKey{})
	authenticator := NewAuthenticator(repo)

	_, err := authenticator.Authenticate(context.Background(), "")
	if !errors.Is(err, ErrMissingAPIKey) {
		t.Fatalf("expected ErrMissingAPIKey, got %v", err)
	}
}

func TestAuthenticatorInvalidKey(t *testing.T) {
	repo, _ := NewInMemoryRepository([]APIKey{})
	authenticator := NewAuthenticator(repo)

	_, err := authenticator.Authenticate(context.Background(), "Bearer sk-invalid")
	if !errors.Is(err, ErrInvalidAPIKey) {
		t.Fatalf("expected ErrInvalidAPIKey, got %v", err)
	}
}

func TestAuthenticatorDisabledKey(t *testing.T) {
	repo, _ := NewInMemoryRepository([]APIKey{
		{
			ID:        "1",
			Name:      "disabled",
			KeyPrefix: KeyPrefix("sk-disabled"),
			KeyHash:   HashAPIKey("sk-disabled"),
			Status:    APIKeyStatusDisabled,
		},
	})
	authenticator := NewAuthenticator(repo)

	_, err := authenticator.Authenticate(context.Background(), "Bearer sk-disabled")
	if !errors.Is(err, ErrDisabledAPIKey) {
		t.Fatalf("expected ErrDisabledAPIKey, got %v", err)
	}
}

func TestAuthenticatorRevokedKey(t *testing.T) {
	repo, _ := NewInMemoryRepository([]APIKey{
		{
			ID:        "1",
			Name:      "revoked",
			KeyPrefix: KeyPrefix("sk-revoked"),
			KeyHash:   HashAPIKey("sk-revoked"),
			Status:    APIKeyStatusRevoked,
		},
	})
	authenticator := NewAuthenticator(repo)

	_, err := authenticator.Authenticate(context.Background(), "Bearer sk-revoked")
	if !errors.Is(err, ErrRevokedAPIKey) {
		t.Fatalf("expected ErrRevokedAPIKey, got %v", err)
	}
}

func TestAuthenticatorExpiredStatusKey(t *testing.T) {
	repo, _ := NewInMemoryRepository([]APIKey{
		{
			ID:        "1",
			Name:      "expired",
			KeyPrefix: KeyPrefix("sk-expired"),
			KeyHash:   HashAPIKey("sk-expired"),
			Status:    APIKeyStatusExpired,
		},
	})
	authenticator := NewAuthenticator(repo)

	_, err := authenticator.Authenticate(context.Background(), "Bearer sk-expired")
	if !errors.Is(err, ErrExpiredAPIKey) {
		t.Fatalf("expected ErrExpiredAPIKey, got %v", err)
	}
}

func TestAuthenticatorExpiredAtKey(t *testing.T) {
	expiresAt := time.Now().UTC().Add(-time.Minute)
	repo, _ := NewInMemoryRepository([]APIKey{
		{
			ID:        "1",
			Name:      "expired",
			KeyPrefix: KeyPrefix("sk-expired-at"),
			KeyHash:   HashAPIKey("sk-expired-at"),
			Status:    APIKeyStatusActive,
			ExpiresAt: &expiresAt,
		},
	})
	authenticator := NewAuthenticator(repo)

	_, err := authenticator.Authenticate(context.Background(), "Bearer sk-expired-at")
	if !errors.Is(err, ErrExpiredAPIKey) {
		t.Fatalf("expected ErrExpiredAPIKey, got %v", err)
	}
}
