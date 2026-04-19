BINARY_NAME=immortal
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
MODULE=github.com/Nagendhra-web/Immortal
LDFLAGS=-ldflags "-s -w -X $(MODULE)/internal/version.Version=$(VERSION) -X $(MODULE)/internal/version.GitCommit=$(COMMIT) -X $(MODULE)/internal/version.BuildDate=$(DATE)"

.PHONY: build test clean lint run

build:
	go build $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/immortal

test:
	go test -v -race -coverprofile=coverage.out ./...

coverage: test
	go tool cover -html=coverage.out -o coverage.html

clean:
	rm -rf bin/ dist/ coverage.out coverage.html

lint:
	golangci-lint run ./...

run: build
	./bin/$(BINARY_NAME)
