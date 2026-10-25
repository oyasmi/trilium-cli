VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null)
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT)

.PHONY: build test lint e2e tokens cover clean

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o trilium ./cmd/trilium

test:
	go vet ./...
	go test ./...

lint:
	golangci-lint run

e2e:
	TRILIUM_E2E=1 go test -count=1 -v ./e2e/...

tokens: build
	./scripts/token-audit.sh

cover:
	go test -coverprofile=coverage.out ./internal/...
	go tool cover -func=coverage.out | tail -1

clean:
	rm -rf trilium dist coverage.out
