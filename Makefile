build-cli:
	go build -o driftmap ./cmd/cli

build-api:
	go build -o driftmap-api ./cmd/api

test:
	go test ./...
