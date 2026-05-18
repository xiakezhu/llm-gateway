# Local LLM Gateway (Phase 3)

Phase 2 provides:

- `GET /health`
- `POST /v1/chat/completions` non-streaming
- `POST /v1/chat/completions` streaming SSE (`stream=true`, llama.cpp backend)
- Basic request validation
- Request ID propagation/generation via `X-Request-ID`
- OpenAI-compatible backend abstraction (`internal/backend`)
- Exact model-name routing (`internal/router`)
- API key authentication middleware (`Authorization: Bearer <key>`)
- Optional OpenAI provider backend with provider API key injection

## Run

```bash
make run
```

The gateway auto-loads `.env` from project root on startup (if present). Existing shell environment variables take precedence over values in `.env`.

Default environment variables:

- `GATEWAY_ADDR=:8080`
- `GATEWAY_DEFAULT_MODEL=local-llama`
- `LLAMA_CPP_BASE_URL=http://localhost:8081`
- `LLAMA_CPP_API_KEY_ENV=LLAMA_CPP_API_KEY`
- `GATEWAY_BACKEND_TIMEOUT_SECONDS=60`
- `GATEWAY_OPENAI_ENABLED=false`
- `OPENAI_MODEL_NAME=gpt-4o-mini`
- `OPENAI_BASE_URL=https://api.openai.com/v1`
- `OPENAI_API_KEY_ENV=OPENAI_API_KEY`
- `GATEWAY_AUTH_ENABLED=true`
- `GATEWAY_BOOTSTRAP_API_KEY=sk-local-demo`
- `GATEWAY_BOOTSTRAP_API_KEY_NAME=local-demo`

When `GATEWAY_OPENAI_ENABLED=true`, the gateway reads provider credentials from the env var named by `OPENAI_API_KEY_ENV` and sends `Authorization: Bearer <provider_key>` to the model provider.

For llama.cpp, if `LLAMA_CPP_API_KEY` is set (or whichever variable is named by `LLAMA_CPP_API_KEY_ENV`), the gateway also sends `Authorization: Bearer <provider_key>` to llama.cpp.

## Test

```bash
make test
```

## Example request

```bash
curl -s http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-local-demo" \
  -d '{
    "model":"local-llama",
    "messages":[{"role":"user","content":"Explain LLM gateways"}]
  }'
```

If your llama.cpp server is OpenAI-compatible, the gateway forwards this request to:

`$LLAMA_CPP_BASE_URL/v1/chat/completions`

If OpenAI backend is enabled and request model is `OPENAI_MODEL_NAME`, the gateway forwards to:

`$OPENAI_BASE_URL/chat/completions`

## Streaming example

```bash
curl -N http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-local-demo" \
  -d '{
    "model":"local-llama",
    "stream":true,
    "messages":[{"role":"user","content":"Explain LLM gateways"}]
  }'
```

## Python API test script

```bash
pip install openai
python scripts/test_chat_api.py
```
