package auth

import (
	"context"
	"errors"
	"testing"
)

func TestAuthenticatorSuccess(t *testing.T) {
	repo, err := NewInMemoryRepository([]APIKey{
		{
			ID:      "1",
			Name:    "demo",
			KeyHash: HashAPIKey("sk-valid"),
			Enabled: true,
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
			ID:      "1",
			Name:    "disabled",
			KeyHash: HashAPIKey("sk-disabled"),
			Enabled: false,
		},
	})
	authenticator := NewAuthenticator(repo)

	_, err := authenticator.Authenticate(context.Background(), "Bearer sk-disabled")
	if !errors.Is(err, ErrDisabledAPIKey) {
		t.Fatalf("expected ErrDisabledAPIKey, got %v", err)
	}
}
