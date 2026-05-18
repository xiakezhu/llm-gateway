run:
	go run ./cmd/gateway

test:
	go test ./...

lint:
	go vet ./...

fmt:
	go fmt ./...
