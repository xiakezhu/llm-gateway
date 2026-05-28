run:
	go run ./cmd/gateway

test:
	go test ./...

lint:
	go vet ./...

fmt:
	go fmt ./...

openwebui-up:
	docker compose -f docker-compose.openwebui.yml up -d

openwebui-down:
	docker compose -f docker-compose.openwebui.yml down

openwebui-logs:
	docker compose -f docker-compose.openwebui.yml logs -f open-webui
