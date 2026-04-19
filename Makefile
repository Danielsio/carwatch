.PHONY: build run test test-cover test-e2e lint ci clean docker-build docker-run

COVER_DIR := .coverage
COVER_PROFILE := $(COVER_DIR)/coverage.out

# Only test packages that have test files
TEST_PKGS := $(shell go list ./... | xargs -I{} sh -c 'go list -f "{{if .TestGoFiles}}{{.ImportPath}}{{end}}" {}' | grep .)

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_TIME ?= $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS := -ldflags "-s -w \
	-X main.version=$(VERSION) \
	-X main.gitCommit=$(GIT_COMMIT) \
	-X main.buildTime=$(BUILD_TIME)"

build:
	go build $(LDFLAGS) -o bot ./cmd/bot

run: build
	./bot -config config.yaml

test:
	@mkdir -p $(COVER_DIR)
	go test -count=1 -coverprofile=$(COVER_PROFILE) -covermode=atomic $(TEST_PKGS)
	@echo ""
	@echo "=== Coverage Summary ==="
	@go tool cover -func=$(COVER_PROFILE) | tail -1
	@echo "HTML report: make test-cover"

test-cover: test
	go tool cover -html=$(COVER_PROFILE) -o $(COVER_DIR)/coverage.html
	@echo "Coverage report: $(COVER_DIR)/coverage.html"

test-e2e:
	go test -count=1 -v -tags=e2e ./e2e/...

lint:
	golangci-lint run ./...

ci: lint test

clean:
	rm -f bot
	rm -rf $(COVER_DIR)

docker-build:
	docker build -t carwatch .

docker-run:
	docker compose up -d
