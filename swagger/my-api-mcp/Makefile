.PHONY: generate build test clean

generate:
	oapi-codegen -config oapi-codegen.yaml api/spec.yaml

build: generate
	go build -o bin/client cmd/client/main.go
	go build -o bin/mcp-server cmd/server/main.go

test:
	go test ./...

clean:
	rm -rf bin/ gen/

run-server: build
	./bin/mcp-server

run-client: build
	./bin/client

dev:
	go run cmd/server/main.go