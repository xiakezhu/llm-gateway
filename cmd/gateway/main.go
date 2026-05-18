package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"local-llm-gateway/internal/api"
	"local-llm-gateway/internal/auth"
	"local-llm-gateway/internal/backend"
	"local-llm-gateway/internal/config"
	"local-llm-gateway/internal/router"
)

func main() {
	if err := config.LoadDotEnv(".env"); err != nil {
		log.Printf("warning: failed to load .env: %v", err)
	}

	cfg := config.LoadFromEnv()

	modelRoutes := make(map[string]backend.Backend)

	llamaBackend := backend.NewLlamaCPPBackend(
		"llama.cpp",
		cfg.LlamaCPPBaseURL,
		cfg.LlamaCPPAPIKey,
		time.Duration(cfg.BackendTimeoutSeconds)*time.Second,
	)
	modelRoutes[cfg.DefaultModelName] = llamaBackend

	if cfg.OpenAIEnabled {
		if cfg.OpenAIAPIKey == "" {
			log.Printf("openai backend enabled but provider key is empty (expected env var: %s)", cfg.OpenAIAPIKeyEnv)
			os.Exit(1)
		}

		openAIBackend := backend.NewOpenAIBackend(
			"openai",
			cfg.OpenAIBaseURL,
			cfg.OpenAIAPIKey,
			time.Duration(cfg.BackendTimeoutSeconds)*time.Second,
		)
		modelRoutes[cfg.OpenAIModelName] = openAIBackend
	}

	modelRouter, err := router.NewModelRouter(modelRoutes)
	if err != nil {
		log.Printf("failed to initialize model router: %v", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	handler := api.NewHandler(modelRouter)
	handler.RegisterRoutes(mux)

	finalHandler := http.Handler(mux)
	if cfg.AuthEnabled {
		authRepo, err := auth.NewInMemoryRepository([]auth.APIKey{
			{
				ID:       "key_local_demo",
				Name:     cfg.BootstrapAPIKeyName,
				KeyHash:  auth.HashAPIKey(cfg.BootstrapAPIKey),
				Enabled:  true,
				RPMLimit: 60,
				TPMLimit: 60000,
			},
		})
		if err != nil {
			log.Printf("failed to initialize auth repository: %v", err)
			os.Exit(1)
		}

		authenticator := auth.NewAuthenticator(authRepo)
		finalHandler = api.APIKeyAuthMiddleware(authenticator, finalHandler)
	}

	server := &http.Server{
		Addr:    cfg.ServerAddr,
		Handler: api.RequestIDMiddleware(finalHandler),
	}

	log.Printf("gateway listening on %s", cfg.ServerAddr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("server error: %v", err)
		os.Exit(1)
	}
}
