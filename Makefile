.PHONY: all build run test test-cover test-e2e lint ci clean docker-build docker-run \
       vm-check-env vm-ssh vm-logs vm-restart vm-stop vm-start vm-status vm-deploy \
       web-install web-dev web-build

all: build

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

web-install:
	cd web && npm install

web-dev:
	cd web && npm run dev

web-build:
	cd web && npm run build

docker-build:
	docker build -t carwatch .

docker-run:
	docker compose -f docker-compose.dev.yaml up -d

# --- VM Management ---
# Set these in your shell profile (~/.bashrc or ~/.zshrc):
#   export CARWATCH_VM_IP=129.159.142.247
#   export CARWATCH_VM_KEY=~/Downloads/ssh-key-2026-04-20.key
#   export CARWATCH_VM_USER=ubuntu

VM_IP   := $(CARWATCH_VM_IP)
VM_KEY  := $(CARWATCH_VM_KEY)
VM_USER := $(or $(CARWATCH_VM_USER),ubuntu)
SSH     := ssh -i $(VM_KEY) $(VM_USER)@$(VM_IP)

vm-check-env:
	@test -n "$(VM_IP)"  || (echo "Error: set CARWATCH_VM_IP";  exit 1)
	@test -n "$(VM_KEY)" || (echo "Error: set CARWATCH_VM_KEY"; exit 1)
	@test -r "$(VM_KEY)" || (echo "Error: CARWATCH_VM_KEY is not readable: $(VM_KEY)"; exit 1)

vm-ssh: vm-check-env
	$(SSH)

vm-logs: vm-check-env
	$(SSH) "docker logs carwatch --tail 50"

vm-status: vm-check-env
	$(SSH) "docker ps --filter name=carwatch && echo '---' && docker exec carwatch /bot -version"

vm-stop: vm-check-env
	$(SSH) "docker stop carwatch"

vm-start: vm-check-env
	$(SSH) "docker start carwatch"

vm-restart: vm-check-env
	$(SSH) "docker restart carwatch"

vm-deploy: vm-check-env
	$(SSH) "docker pull ghcr.io/danielsio/carwatch:latest \
		&& (docker network create carwatch-net >/dev/null 2>&1 || true) \
		&& (docker volume create carwatch_carwatch-data >/dev/null 2>&1 || true) \
		&& (docker stop carwatch >/dev/null 2>&1 || true) \
		&& (docker rm carwatch >/dev/null 2>&1 || true) \
		&& docker run -d --name carwatch --restart unless-stopped \
			--label com.centurylinklabs.watchtower.enable=true \
			--network carwatch-net \
			-v carwatch_carwatch-data:/data \
			-v /home/$(VM_USER)/carwatch/config.yaml:/config.yaml:ro \
			-v /home/$(VM_USER)/carwatch/firebase-sa.json:/config/firebase-sa.json:ro \
			ghcr.io/danielsio/carwatch:latest \
		&& sleep 3 && docker exec carwatch /bot -version"
