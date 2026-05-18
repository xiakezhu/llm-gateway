# AGENTS.md

## Project: OpenAI-compatible Local LLM Gateway

This project is a Go-based AI infrastructure gateway that exposes an OpenAI-compatible `/v1/chat/completions` API and routes requests to local or cloud LLM backends such as llama.cpp, vLLM, Ollama, OpenAI-compatible providers, or other HTTP inference servers.

The goal is not to build a simple chatbot. The goal is to build a production-style LLM serving gateway with authentication, streaming, model routing, fallback, rate limiting, observability, and prompt/request logging.

---

## Core Product Goal

Build a lightweight OpenAI-compatible LLM Gateway that allows clients, agents, dashboards, or SDKs to use a single API endpoint:

```http
POST /v1/chat/completions
````

The gateway should support:

* OpenAI-compatible request and response format
* Streaming responses via Server-Sent Events
* Multiple backend providers
* API key authentication
* Per-key rate limiting
* Model routing
* Fallback to cloud models
* Request, latency, token, and error metrics
* Prompt and response logging
* React dashboard for observability and management

---

## Preferred Tech Stack

Use the following stack unless the user explicitly asks otherwise:

### Backend

* Go
* Standard library where reasonable
* `net/http` or a lightweight router such as `chi`
* SQLite for local-first MVP
* PostgreSQL as optional production DB
* Prometheus client for metrics
* Docker and Docker Compose

### Frontend

* React
* TypeScript
* Vite
* Simple dashboard UI
* Charts for request volume, latency, token usage, and fallback rate

### LLM Backends

The gateway should be designed to support:

* llama.cpp server
* vLLM OpenAI-compatible server
* Ollama, if useful
* OpenAI-compatible cloud APIs
* Future custom providers

---

## High-Level Architecture

```txt
Client / Agent / OpenAI SDK
        |
        v
OpenAI-compatible Gateway
        |
        +--> Auth Middleware
        +--> Rate Limit Middleware
        +--> Request Validation
        +--> Model Router
        +--> Backend Adapter
        +--> Metrics Collector
        +--> Prompt Logger
        |
        +--> llama.cpp
        +--> vLLM
        +--> Ollama
        +--> Cloud Provider Fallback
```

---

## Main API Contract

The primary API is:

```http
POST /v1/chat/completions
```

The request should be compatible with OpenAI's chat completions format:

```json
{
  "model": "local-llama",
  "messages": [
    {
      "role": "user",
      "content": "Explain Kubernetes in simple terms."
    }
  ],
  "temperature": 0.7,
  "max_tokens": 512,
  "stream": true
}
```

Non-streaming response should look like:

```json
{
  "id": "chatcmpl_xxx",
  "object": "chat.completion",
  "created": 1710000000,
  "model": "local-llama",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "..."
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 100,
    "completion_tokens": 200,
    "total_tokens": 300
  }
}
```

Streaming response should use Server-Sent Events:

```txt
data: {"id":"chatcmpl_xxx","object":"chat.completion.chunk","choices":[{"delta":{"content":"Hello"}}]}

data: {"id":"chatcmpl_xxx","object":"chat.completion.chunk","choices":[{"delta":{"content":" world"}}]}

data: [DONE]
```

---

## Recommended Repository Structure

Use this structure unless there is a clear reason to change it:

```txt
local-llm-gateway/
  AGENTS.md
  README.md
  docker-compose.yml
  Makefile
  .env.example

  cmd/
    gateway/
      main.go

  internal/
    api/
      handler.go
      middleware.go
      openai_types.go
      errors.go

    auth/
      api_key.go
      hash.go
      repository.go

    backend/
      interface.go
      llamacpp.go
      vllm.go
      ollama.go
      openai.go
      errors.go

    router/
      model_router.go
      policy.go
      health.go

    ratelimit/
      limiter.go

    metrics/
      prometheus.go

    logs/
      prompt_log.go
      repository.go

    db/
      sqlite.go
      postgres.go
      migrations/

    config/
      config.go

  dashboard/
    package.json
    src/

  docs/
    architecture.md
    api.md
    routing.md
    observability.md
```

---

## Backend Design Rules

### 1. Keep API layer separate from backend providers

The API handler should not directly call llama.cpp, vLLM, OpenAI, or Ollama.

Instead, route all model calls through a backend interface.

Use an interface similar to:

```go
type Backend interface {
    Name() string
    Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
    ChatStream(ctx context.Context, req ChatRequest) (<-chan ChatChunk, error)
    Health(ctx context.Context) error
}
```

The model router should decide which backend to use.

---

### 2. Preserve OpenAI compatibility

When adding or modifying API types, keep field names compatible with OpenAI-style JSON.

Prefer this:

```go
type ChatCompletionRequest struct {
    Model       string        `json:"model"`
    Messages    []ChatMessage `json:"messages"`
    Temperature *float64      `json:"temperature,omitempty"`
    MaxTokens   *int          `json:"max_tokens,omitempty"`
    Stream      bool          `json:"stream,omitempty"`
}
```

Avoid inventing custom request fields unless they are isolated under a gateway-specific namespace, for example:

```json
{
  "gateway_options": {
    "allow_fallback": true,
    "preferred_backend": "llama.cpp"
  }
}
```

---

### 3. Streaming must use SSE

Streaming should follow OpenAI-style `data:` events.

Required headers:

```http
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive
```

Each chunk should be flushed immediately.

Do not buffer the full response before streaming.

---

### 4. Do not leak internal backend errors directly

Return safe user-facing errors.

Internal errors should be logged with request ID and backend name.

For example:

```json
{
  "error": {
    "message": "The selected model backend is temporarily unavailable.",
    "type": "backend_unavailable",
    "code": "backend_unavailable"
  }
}
```

---

### 5. Every request should have a request ID

Generate or propagate a request ID for each request.

Use it for:

* logs
* prompt logs
* metrics labels when appropriate
* error debugging
* dashboard request detail page

Prefer accepting an incoming header:

```http
X-Request-ID
```

If missing, generate one.

---

## Model Routing Rules

The gateway should support configurable model routing.

Example config:

```yaml
models:
  - name: "local-llama"
    backend: "llama.cpp"
    url: "http://llama-cpp:8080"
    max_context: 8192
    priority: 1
    fallback_models:
      - "gpt-4o-mini"

  - name: "fast-vllm"
    backend: "vllm"
    url: "http://vllm:8000"
    max_context: 32768
    priority: 1

  - name: "gpt-4o-mini"
    backend: "openai"
    url: "https://api.openai.com/v1"
    priority: 10
```

Routing should eventually support:

* exact model name routing
* backend health-aware routing
* fallback routing
* context-length-aware routing
* simple priority-based routing
* future weighted routing

MVP can start with exact model name routing.

---

## Fallback Rules

Fallback should be supported, but it must be explicit and observable.

Fallback may happen when:

* local backend is unavailable
* backend timeout occurs
* backend returns retryable 5xx error
* context length exceeds local model capacity
* model policy allows cloud fallback

Fallback should not happen silently.

Log fallback metadata:

```json
{
  "fallback_used": true,
  "original_model": "local-llama",
  "fallback_model": "gpt-4o-mini",
  "fallback_reason": "backend_timeout"
}
```

Expose fallback metrics:

```txt
llm_gateway_fallbacks_total
```

---

## API Key Authentication

Support API key auth using:

```http
Authorization: Bearer sk-local-xxx
```

Do not store raw API keys.

Store hashed keys only.

Recommended table:

```sql
CREATE TABLE api_keys (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    key_hash TEXT NOT NULL UNIQUE,
    rpm_limit INTEGER NOT NULL DEFAULT 60,
    tpm_limit INTEGER NOT NULL DEFAULT 60000,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at DATETIME NOT NULL,
    disabled_at DATETIME
);
```

Behavior:

* missing key -> `401 Unauthorized`
* invalid key -> `401 Unauthorized`
* disabled key -> `403 Forbidden`
* over rate limit -> `429 Too Many Requests`

---

## Rate Limiting

MVP rate limiting can be in-memory.

Use per-API-key limits.

Support:

* requests per minute
* optional tokens per minute

Initial implementation can use Go's token bucket:

```go
golang.org/x/time/rate
```

Do not make rate limiting global only. It should be associated with API keys.

---

## Metrics Requirements

Expose Prometheus metrics at:

```http
GET /metrics
```

Recommended metrics:

```txt
llm_gateway_requests_total
llm_gateway_request_duration_seconds
llm_gateway_tokens_total
llm_gateway_prompt_tokens_total
llm_gateway_completion_tokens_total
llm_gateway_errors_total
llm_gateway_fallbacks_total
llm_gateway_active_streams
llm_gateway_backend_health
```

Useful labels:

```txt
model
backend
status
stream
fallback
error_type
```

Avoid high-cardinality labels such as raw prompt, user input, request ID, or API key value.

For API key label, use a stable key name or key ID only if cardinality is controlled.

---

## Prompt Logging

Prompt logs should be structured.

Recommended table:

```sql
CREATE TABLE prompt_logs (
    id TEXT PRIMARY KEY,
    request_id TEXT NOT NULL,
    api_key_id TEXT,
    api_key_name TEXT,
    model TEXT NOT NULL,
    backend TEXT,
    stream BOOLEAN NOT NULL,
    prompt_tokens INTEGER,
    completion_tokens INTEGER,
    total_tokens INTEGER,
    latency_ms INTEGER,
    status TEXT NOT NULL,
    fallback_used BOOLEAN NOT NULL DEFAULT FALSE,
    fallback_model TEXT,
    fallback_reason TEXT,
    error_message TEXT,
    created_at DATETIME NOT NULL
);
```

Optional message table:

```sql
CREATE TABLE prompt_log_messages (
    id TEXT PRIMARY KEY,
    prompt_log_id TEXT NOT NULL,
    role TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at DATETIME NOT NULL
);
```

Prompt content logging should be configurable.

Default for development can be:

```yaml
logging:
  store_prompts: true
  store_responses: true
```

Default for production-like mode should be:

```yaml
logging:
  store_prompts: false
  store_responses: false
```

Support redaction in the future for:

* emails
* phone numbers
* API keys
* bearer tokens
* passwords
* secrets

---

## Dashboard Requirements

The dashboard should show operational information, not just a chat UI.

Recommended pages:

### Overview

Show:

* total requests
* success rate
* error rate
* average latency
* P95 latency
* token usage
* fallback rate
* active streams

### Requests

Show request logs:

* request ID
* timestamp
* model
* backend
* status
* latency
* prompt tokens
* completion tokens
* fallback used

### Models

Show:

* model name
* backend type
* backend URL
* health status
* context window
* fallback policy

### API Keys

Show:

* key name
* enabled status
* RPM limit
* TPM limit
* recent usage

### Prompt Detail

Show one request's detail:

* messages, if enabled
* response, if enabled
* metadata
* error
* fallback information

---

## Docker Compose Requirements

The project should support local development using Docker Compose.

Recommended services:

```txt
gateway
sqlite or postgres
prometheus
grafana
dashboard
llama.cpp-server
```

For MVP, SQLite is acceptable and simpler.

For a more production-like demo, PostgreSQL can be added.

---

## Makefile Commands

Prefer adding a Makefile with commands like:

```makefile
run:
	go run ./cmd/gateway

test:
	go test ./...

lint:
	go vet ./...

docker-up:
	docker compose up --build

docker-down:
	docker compose down

fmt:
	go fmt ./...
```

---

## Testing Expectations

When modifying backend code, add or update tests where practical.

Important test areas:

* OpenAI request parsing
* response formatting
* streaming chunk formatting
* API key validation
* rate limiting behavior
* model routing
* fallback behavior
* prompt logging
* metrics registration

Prefer table-driven Go tests.

---

## Error Handling Style

Use typed errors where helpful.

Examples:

```go
var ErrBackendUnavailable = errors.New("backend unavailable")
var ErrRateLimited = errors.New("rate limited")
var ErrInvalidAPIKey = errors.New("invalid api key")
```

Map internal errors to OpenAI-style API errors.

Example response:

```json
{
  "error": {
    "message": "Rate limit exceeded.",
    "type": "rate_limit_error",
    "code": "rate_limit_exceeded"
  }
}
```

---

## Configuration

Prefer YAML or environment variables.

Example config:

```yaml
server:
  addr: ":8080"

database:
  driver: "sqlite"
  dsn: "./gateway.db"

auth:
  enabled: true

rate_limit:
  default_rpm: 60
  default_tpm: 60000

logging:
  store_prompts: true
  store_responses: true

metrics:
  enabled: true

models:
  - name: "local-llama"
    backend: "llama.cpp"
    url: "http://localhost:8081"
    max_context: 8192
    timeout_seconds: 60
    fallback_models:
      - "gpt-4o-mini"

  - name: "gpt-4o-mini"
    backend: "openai"
    url: "https://api.openai.com/v1"
    timeout_seconds: 60
```

Sensitive values should come from environment variables:

```env
OPENAI_API_KEY=...
DATABASE_URL=...
GATEWAY_MASTER_KEY=...
```

Never commit real secrets.

---

## Coding Style

Use clean, boring, maintainable Go.

Prefer:

* small interfaces
* explicit errors
* context propagation
* structured logging
* dependency injection through constructors
* table-driven tests

Avoid:

* global mutable state unless justified
* leaking provider-specific types into API handlers
* mixing database logic with HTTP handlers
* storing raw API keys
* high-cardinality Prometheus labels
* logging sensitive prompt content without configuration

---

## Agent Task Guidelines

When implementing a new feature:

1. Read this file first.
2. Check existing structure before creating new packages.
3. Preserve OpenAI API compatibility.
4. Keep backend-specific logic behind adapter interfaces.
5. Add or update tests when practical.
6. Update README or docs if behavior changes.
7. Do not introduce unnecessary frameworks.
8. Prefer simple MVP implementation over over-engineered abstractions.
9. Do not commit secrets, API keys, or local database files.
10. Make sure the project still builds with:

```bash
go test ./...
```

---

## Suggested Implementation Milestones

### Milestone 1: Basic Gateway

* Start Go HTTP server
* Add health endpoint
* Add `/v1/chat/completions`
* Return mock OpenAI-compatible response
* Add basic request validation

### Milestone 2: llama.cpp Backend

* Add backend interface
* Implement llama.cpp adapter
* Route requests to llama.cpp server
* Return normalized OpenAI-compatible response

### Milestone 3: Streaming

* Add `stream: true` support
* Implement SSE response writer
* Normalize streaming chunks

### Milestone 4: API Key Auth

* Add API key middleware
* Store hashed API keys
* Add SQLite table
* Reject missing or invalid keys

### Milestone 5: Rate Limit

* Add per-key RPM limit
* Return 429 on excessive requests
* Add tests

### Milestone 6: Metrics

* Add Prometheus endpoint
* Track request count, latency, token usage, errors, fallback count
* Add Docker Compose Prometheus config

### Milestone 7: Prompt Logs

* Add request logging table
* Store metadata
* Make prompt/response content storage configurable

### Milestone 8: Model Routing

* Add model config
* Route by model name
* Track backend health
* Prepare for fallback policy

### Milestone 9: Cloud Fallback

* Add OpenAI-compatible cloud backend
* Fallback when local backend fails
* Log fallback reason
* Expose fallback metrics

### Milestone 10: React Dashboard

* Show overview metrics
* Show request logs
* Show model health
* Show API key usage

---

## Resume-Oriented Outcome

The final project should be presentable as:

> Built a Go-based OpenAI-compatible LLM serving gateway supporting streaming chat completions, API key authentication, per-key rate limiting, configurable model routing across llama.cpp/vLLM/cloud providers, automatic fallback, prompt logging, and Prometheus/Grafana observability.

This project should demonstrate understanding of:

* LLM serving infrastructure
* gateway design
* OpenAI-compatible API design
* streaming protocols
* model routing
* local inference backends
* fallback and reliability
* observability
* secure API access
* production-style backend engineering

---

## Non-Goals for MVP

Do not prioritize these in the first version:

* full user management system
* billing system
* complex Kubernetes deployment
* fine-tuning
* vector database / RAG pipeline
* multi-agent orchestration
* advanced distributed scheduling
* enterprise RBAC
* complicated frontend styling

These can be added later, but the MVP should focus on the gateway and serving infrastructure.

---

## Definition of Done

A strong MVP is complete when:

* OpenAI SDK can call this gateway using a custom base URL
* `/v1/chat/completions` supports non-streaming and streaming
* at least one local backend works
* one cloud fallback backend works
* API key auth works
* per-key rate limiting works
* request metrics are exposed to Prometheus
* prompt/request logs are stored
* Docker Compose can start the core system
* README explains how to run and test the gateway

Example client usage should work:

```python
from openai import OpenAI

client = OpenAI(
    api_key="sk-local-demo",
    base_url="http://localhost:8080/v1"
)

response = client.chat.completions.create(
    model="local-llama",
    messages=[
        {"role": "user", "content": "Explain LLM gateways."}
    ]
)

print(response.choices[0].message.content)
```
