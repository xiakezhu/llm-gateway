package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrMissingAPIKey  = errors.New("missing api key")
	ErrInvalidAPIKey  = errors.New("invalid api key")
	ErrDisabledAPIKey = errors.New("disabled api key")
	ErrRevokedAPIKey  = errors.New("revoked api key")
	ErrExpiredAPIKey  = errors.New("expired api key")
)

const (
	APIKeyStatusActive   = "active"
	APIKeyStatusDisabled = "disabled"
	APIKeyStatusRevoked  = "revoked"
	APIKeyStatusExpired  = "expired"
)

type APIKey struct {
	ID         string
	Name       string
	KeyPrefix  string
	KeyHash    string
	RPMLimit   int
	TPMLimit   int
	Status     string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	ExpiresAt  *time.Time
	DisabledAt *time.Time
}

type Authenticator struct {
	repo Repository
}

func NewAuthenticator(repo Repository) *Authenticator {
	return &Authenticator{repo: repo}
}

func (a *Authenticator) Authenticate(ctx context.Context, authorization string) (*APIKey, error) {
	rawKey, err := parseBearerToken(authorization)
	if err != nil {
		return nil, err
	}

	keyHash := HashAPIKey(rawKey)
	apiKey, err := a.repo.FindByHash(ctx, keyHash)
	if err != nil {
		if errors.Is(err, ErrAPIKeyNotFound) {
			return nil, ErrInvalidAPIKey
		}
		return nil, fmt.Errorf("auth repository lookup failed: %w", err)
	}

	switch apiKey.Status {
	case "", APIKeyStatusActive:
	case APIKeyStatusDisabled:
		return nil, ErrDisabledAPIKey
	case APIKeyStatusRevoked:
		return nil, ErrRevokedAPIKey
	case APIKeyStatusExpired:
		return nil, ErrExpiredAPIKey
	default:
		return nil, ErrInvalidAPIKey
	}

	if apiKey.ExpiresAt != nil && !apiKey.ExpiresAt.After(time.Now().UTC()) {
		return nil, ErrExpiredAPIKey
	}

	return apiKey, nil
}

func parseBearerToken(authorization string) (string, error) {
	authorization = strings.TrimSpace(authorization)
	if authorization == "" {
		return "", ErrMissingAPIKey
	}

	const prefix = "Bearer "
	if !strings.HasPrefix(authorization, prefix) {
		return "", ErrInvalidAPIKey
	}

	token := strings.TrimSpace(strings.TrimPrefix(authorization, prefix))
	if token == "" {
		return "", ErrInvalidAPIKey
	}

	return token, nil
}
