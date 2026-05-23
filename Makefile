.PHONY: all build build-api build-scraper build-notifier build-all \
       run test test-cover test-e2e lint ci clean docker-build docker-run \
       dev dev-db dev-stop dev-reset dev-pg-shell \
       vm-check-env vm-ssh vm-logs logs vm-restart vm-stop vm-start vm-status vm-deploy vm-deploy-all vm-sync \
       vm-backup vm-backup-list vm-pg-shell \
       vm-setup-backup vm-backup-status \
       web-install web-dev web-build \
       catalog-refresh

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

build-api:
	go build $(LDFLAGS) -o api-server ./cmd/api-server

build-scraper:
	go build $(LDFLAGS) -o scraper ./cmd/scraper

build-notifier:
	go build $(LDFLAGS) -o notifier-worker ./cmd/notifier

build-all: build build-api build-scraper build-notifier

catalog-refresh:
	go run ./cmd/catalog-gen -output internal/catalog/catalog_data.json

run: build
	./bot -config config.yaml

test:
	@mkdir -p $(COVER_DIR)
	go build ./...
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
	rm -f bot api-server scraper notifier-worker
	rm -rf $(COVER_DIR)

web-install:
	cd web && npm install

web-dev:
	cd web && npm run dev

web-build:
	cd web && npm run build

dev-db:
	docker compose -f docker-compose.dev.yaml up -d
	@echo "PostgreSQL running on localhost:5432 (user: carwatch, pass: carwatch, db: carwatch)"

dev-stop:
	docker compose -f docker-compose.dev.yaml down

dev-reset:
	docker compose -f docker-compose.dev.yaml down -v
	@echo "Database volume removed. Run 'make dev-db' to start fresh."

dev-pg-shell:
	docker exec -it carwatch-dev-pg psql -U carwatch carwatch

dev: build dev-db
	./bot -config config.dev.yaml

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
	$(SSH) "docker logs carwatch --tail 200 -f"

LOGS_FILTER ?=
logs: vm-check-env
ifdef LOGS_FILTER
	$(SSH) "docker logs carwatch --tail 500 -f 2>&1" | grep -iF --line-buffered -- "$(LOGS_FILTER)"
else
	$(SSH) "docker logs carwatch --tail 200 -f"
endif

vm-status: vm-check-env
	$(SSH) "docker ps --filter name=carwatch && echo '---' && docker exec carwatch /bot -version"

vm-stop: vm-check-env
	$(SSH) "docker stop carwatch"

vm-start: vm-check-env
	$(SSH) "docker start carwatch"

vm-restart: vm-check-env
	$(SSH) "docker restart carwatch"

SCP     := scp -i $(VM_KEY)
VM_DIR  := /home/$(VM_USER)/carwatch
VM_COMPOSE := cd $(VM_DIR) && docker compose -f docker-compose.prod.yaml

vm-sync: vm-check-env
	$(SSH) "mkdir -p $(VM_DIR)"
	$(SCP) docker-compose.prod.yaml $(VM_USER)@$(VM_IP):$(VM_DIR)/docker-compose.prod.yaml

vm-deploy: vm-sync
	$(SSH) "$(VM_COMPOSE) pull carwatch && $(VM_COMPOSE) up -d --force-recreate carwatch \
		&& sleep 3 && docker exec carwatch /bot -version"

vm-deploy-all: vm-sync
	$(SSH) "$(VM_COMPOSE) pull && $(VM_COMPOSE) up -d \
		&& sleep 3 && docker exec carwatch /bot -version"

vm-backup: vm-check-env
	$(SSH) "mkdir -p ~/carwatch/backups && set -o pipefail && \
		docker exec carwatch-pg pg_dump -U carwatch carwatch | gzip > ~/carwatch/backups/carwatch-\$$(date +%Y%m%d-%H%M%S).sql.gz \
		&& echo 'Backup created' && ls -lhS ~/carwatch/backups/"

vm-backup-list: vm-check-env
	$(SSH) "ls -lhS ~/carwatch/backups/ 2>/dev/null || echo 'No backups found'"

vm-pg-shell: vm-check-env
	$(SSH) -t "docker exec -it carwatch-pg psql -U carwatch carwatch"

vm-setup-backup: vm-check-env
	$(SSH) "mkdir -p $(VM_DIR)/scripts"
	$(SCP) scripts/backup-pg.sh $(VM_USER)@$(VM_IP):$(VM_DIR)/scripts/backup-pg.sh
	$(SCP) scripts/carwatch-backup.service $(VM_USER)@$(VM_IP):$(VM_DIR)/scripts/carwatch-backup.service
	$(SCP) scripts/carwatch-backup.timer $(VM_USER)@$(VM_IP):$(VM_DIR)/scripts/carwatch-backup.timer
	$(SSH) "chmod +x $(VM_DIR)/scripts/backup-pg.sh \
		&& sudo cp $(VM_DIR)/scripts/carwatch-backup.service /etc/systemd/system/ \
		&& sudo cp $(VM_DIR)/scripts/carwatch-backup.timer /etc/systemd/system/ \
		&& sudo systemctl daemon-reload \
		&& sudo systemctl enable --now carwatch-backup.timer \
		&& echo 'Backup timer installed and enabled' \
		&& systemctl list-timers carwatch-backup.timer"

vm-backup-status: vm-check-env
	$(SSH) "systemctl list-timers carwatch-backup.timer \
		&& echo '---' \
		&& echo 'Last 5 backups:' \
		&& ls -lht ~/carwatch/backups/carwatch-backup-*.sql.gz 2>/dev/null | head -5 || echo 'No backups found'"
