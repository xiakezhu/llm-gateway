package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	ServerAddr            string
	DatabaseDriver        string
	DatabaseDSN           string
	DefaultModelName      string
	LlamaCPPBaseURL       string
	LlamaCPPAPIKeyEnv     string
	LlamaCPPAPIKey        string
	BackendTimeoutSeconds int
	OpenAIEnabled         bool
	OpenAIModelName       string
	OpenAIBaseURL         string
	OpenAIAPIKeyEnv       string
	OpenAIAPIKey          string
	AuthEnabled           bool
	BootstrapAPIKey       string
	BootstrapAPIKeyName   string
}

func LoadFromEnv() Config {
	addr := os.Getenv("GATEWAY_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	databaseDriver := os.Getenv("DATABASE_DRIVER")
	if databaseDriver == "" {
		databaseDriver = "sqlite"
	}

	databaseDSN := os.Getenv("DATABASE_DSN")
	if databaseDSN == "" {
		databaseDSN = "./gateway.db"
	}

	modelName := os.Getenv("GATEWAY_DEFAULT_MODEL")
	if modelName == "" {
		modelName = "local-llama"
	}

	llamaURL := os.Getenv("LLAMA_CPP_BASE_URL")
	if llamaURL == "" {
		llamaURL = "http://localhost:8081"
	}
	llamaAPIKeyEnv := os.Getenv("LLAMA_CPP_API_KEY_ENV")
	if llamaAPIKeyEnv == "" {
		llamaAPIKeyEnv = "LLAMA_CPP_API_KEY"
	}
	llamaAPIKey := os.Getenv(llamaAPIKeyEnv)

	timeout := 60
	timeoutStr := os.Getenv("GATEWAY_BACKEND_TIMEOUT_SECONDS")
	if timeoutStr != "" {
		if parsed, err := strconv.Atoi(timeoutStr); err == nil && parsed > 0 {
			timeout = parsed
		}
	}

	openAIEnabled := false
	openAIEnabledStr := os.Getenv("GATEWAY_OPENAI_ENABLED")
	if openAIEnabledStr != "" {
		normalized := strings.TrimSpace(strings.ToLower(openAIEnabledStr))
		openAIEnabled = normalized != "false" && normalized != "0"
	}

	openAIModelName := os.Getenv("OPENAI_MODEL_NAME")
	if openAIModelName == "" {
		openAIModelName = "gpt-4o-mini"
	}

	openAIBaseURL := os.Getenv("OPENAI_BASE_URL")
	if openAIBaseURL == "" {
		openAIBaseURL = "https://api.openai.com/v1"
	}

	openAIAPIKeyEnv := os.Getenv("OPENAI_API_KEY_ENV")
	if openAIAPIKeyEnv == "" {
		openAIAPIKeyEnv = "OPENAI_API_KEY"
	}
	openAIAPIKey := os.Getenv(openAIAPIKeyEnv)

	authEnabled := true
	authEnabledStr := os.Getenv("GATEWAY_AUTH_ENABLED")
	if authEnabledStr != "" {
		normalized := strings.TrimSpace(strings.ToLower(authEnabledStr))
		authEnabled = normalized != "false" && normalized != "0"
	}

	bootstrapAPIKey := os.Getenv("GATEWAY_BOOTSTRAP_API_KEY")
	if bootstrapAPIKey == "" {
		bootstrapAPIKey = "sk-local-demo"
	}

	bootstrapAPIKeyName := os.Getenv("GATEWAY_BOOTSTRAP_API_KEY_NAME")
	if bootstrapAPIKeyName == "" {
		bootstrapAPIKeyName = "local-demo"
	}

	return Config{
		ServerAddr:            addr,
		DatabaseDriver:        databaseDriver,
		DatabaseDSN:           databaseDSN,
		DefaultModelName:      modelName,
		LlamaCPPBaseURL:       llamaURL,
		LlamaCPPAPIKeyEnv:     llamaAPIKeyEnv,
		LlamaCPPAPIKey:        llamaAPIKey,
		BackendTimeoutSeconds: timeout,
		OpenAIEnabled:         openAIEnabled,
		OpenAIModelName:       openAIModelName,
		OpenAIBaseURL:         openAIBaseURL,
		OpenAIAPIKeyEnv:       openAIAPIKeyEnv,
		OpenAIAPIKey:          openAIAPIKey,
		AuthEnabled:           authEnabled,
		BootstrapAPIKey:       bootstrapAPIKey,
		BootstrapAPIKeyName:   bootstrapAPIKeyName,
	}
}
