BIN := go-test-regroup

# Version information injected at build time.
GIT_COMMIT  := $(shell git rev-parse HEAD 2>/dev/null)
GIT_BRANCH  := $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null)
GIT_STATE   := $(shell if git diff --quiet 2>/dev/null; then echo clean; else echo dirty; fi)
BUILD_DATE  := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
GIT_VERSION := $(shell git describe --tags --always --dirty 2>/dev/null)

LDFLAGS := -X main.gitCommit=$(GIT_COMMIT) \
           -X main.gitBranch=$(GIT_BRANCH) \
           -X main.gitState=$(GIT_STATE) \
           -X main.buildDate=$(BUILD_DATE) \
           -X main.version=$(GIT_VERSION)

.PHONY: build clean test vet fmt lint

build: fmt vet
	go build -ldflags "$(LDFLAGS)" -o $(BIN) .

clean:
	rm -f $(BIN)

test:
	go test ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

lint: vet
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed; skipping"; \
	fi
