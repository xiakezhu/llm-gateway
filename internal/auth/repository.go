package auth

import (
	"context"
	"errors"
	"fmt"
)

var ErrAPIKeyNotFound = errors.New("api key not found")

type Repository interface {
	FindByHash(ctx context.Context, keyHash string) (*APIKey, error)
}

type InMemoryRepository struct {
	byHash map[string]APIKey
}

func NewInMemoryRepository(keys []APIKey) (*InMemoryRepository, error) {
	byHash := make(map[string]APIKey, len(keys))
	for _, key := range keys {
		if key.KeyHash == "" {
			return nil, fmt.Errorf("key_hash cannot be empty for api key %q", key.Name)
		}
		byHash[key.KeyHash] = key
	}

	return &InMemoryRepository{
		byHash: byHash,
	}, nil
}

func (r *InMemoryRepository) FindByHash(ctx context.Context, keyHash string) (*APIKey, error) {
	_ = ctx
	key, ok := r.byHash[keyHash]
	if !ok {
		return nil, ErrAPIKeyNotFound
	}

	result := key
	return &result, nil
}
