package router

import (
	"errors"
	"fmt"

	"local-llm-gateway/internal/backend"
)

var ErrModelNotFound = errors.New("model route not found")

type ModelRouter struct {
	routes map[string]backend.Backend
}

func NewModelRouter(routes map[string]backend.Backend) (*ModelRouter, error) {
	if len(routes) == 0 {
		return nil, fmt.Errorf("model routes cannot be empty")
	}

	cloned := make(map[string]backend.Backend, len(routes))
	for model, b := range routes {
		if model == "" {
			return nil, fmt.Errorf("model route key cannot be empty")
		}
		if b == nil {
			return nil, fmt.Errorf("backend for model %q cannot be nil", model)
		}
		cloned[model] = b
	}

	return &ModelRouter{routes: cloned}, nil
}

func (r *ModelRouter) Resolve(model string) (backend.Backend, error) {
	b, ok := r.routes[model]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrModelNotFound, model)
	}

	return b, nil
}
