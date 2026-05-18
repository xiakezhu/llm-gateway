package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrMissingAPIKey  = errors.New("missing api key")
	ErrInvalidAPIKey  = errors.New("invalid api key")
	ErrDisabledAPIKey = errors.New("disabled api key")
)

type APIKey struct {
	ID       string
	Name     string
	KeyHash  string
	Enabled  bool
	RPMLimit int
	TPMLimit int
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

	if !apiKey.Enabled {
		return nil, ErrDisabledAPIKey
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
