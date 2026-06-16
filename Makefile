.PHONY: all build build-api build-bot-poller build-scraper build-notifier build-enricher build-all \
       run test test-cover test-e2e lint ci clean docker-build docker-run \
       dev dev-db dev-stop dev-reset dev-pg-shell migrate \
       vm-check-env vm-ssh vm-logs logs vm-restart vm-stop vm-start vm-status vm-smoke vm-deploy vm-deploy-all vm-sync \
       vm-backup vm-backup-list vm-pg-shell \
       vm-setup-backup vm-backup-status \
       web-install web-dev web-build \
       catalog-refresh \
       bench bench-fast bench-yad2 bench-profile bench-go

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
	go build $(LDFLAGS) -o api-server ./cmd/api-server

build-api: build

build-bot-poller:
	go build $(LDFLAGS) -o bot-poller ./cmd/bot-poller

build-scraper:
	go build $(LDFLAGS) -o scraper ./cmd/scraper

build-notifier:
	go build $(LDFLAGS) -o notifier-worker ./cmd/notifier

build-enricher:
	go build $(LDFLAGS) -o enricher ./cmd/enricher

build-bench:
	go build $(LDFLAGS) -o bench ./cmd/bench

build-all: build build-bot-poller build-scraper build-notifier build-enricher build-bench

catalog-refresh:
	go run ./cmd/catalog-gen -output internal/catalog/catalog_data.json

run: build
	./api-server -config config.yaml

test:
	@mkdir -p $(COVER_DIR)
	go build ./...
	go test -count=1 -shuffle=on -coverprofile=$(COVER_PROFILE) -covermode=atomic $(TEST_PKGS)
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
	rm -f api-server bot-poller scraper notifier-worker bench
	rm -rf $(COVER_DIR) bench-profiles bench-results.json

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
	./api-server -config config.dev.yaml

migrate:
	go run $(LDFLAGS) ./cmd/api-server -config config.yaml -migrate-only

docker-build:
	docker build -t carwatch .

docker-run:
	docker compose -f docker-compose.dev.yaml up -d

# --- Benchmarks ---

bench: build-bench
	./bench --config config.yaml --output bench-results.json

bench-fast: build-bench
	./bench --config config.yaml --phases percolator,scoring,db-dedup,db-queries,market --output bench-results.json

bench-yad2: build-bench
	./bench --config config.yaml --phases yad2,cycle --yad2-cooldown 60s --output bench-results.json

bench-profile: build-bench
	./bench --config config.yaml --pprof --output bench-results.json

bench-go:
	go test -bench=. -benchmem -count=3 -run='^$$' ./internal/percolator/ ./internal/scoring/

# Cross-compile for Oracle ARM VM and run remotely.
#   make vm-bench              — all phases
#   make vm-bench BENCH_PHASES=percolator,scoring  — specific phases
BENCH_PHASES ?= all
BENCH_ARGS ?=

vm-bench: vm-check-env
	@echo "=== Cross-compiling bench for linux/arm64..."
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o bench-arm64 ./cmd/bench
	@echo "=== Uploading to VM..."
	$(SCP) bench-arm64 $(VM_USER)@$(VM_IP):$(VM_DIR)/bench
	$(SSH) "chmod +x $(VM_DIR)/bench && mkdir -p $(VM_DIR)/bench-output"
	@echo "=== Running benchmark on VM (phases: $(BENCH_PHASES))..."
	$(SSH) "docker run --rm \
		--network carwatch-net \
		--env-file $(VM_DIR)/.env \
		-v $(VM_DIR)/config.yaml:/config.yaml:ro \
		-v $(VM_DIR)/bench:/bench:ro \
		-v $(VM_DIR)/migrations:/migrations:ro \
		-v $(VM_DIR)/bench-output:/output \
		alpine:3.24 \
		/bench --config /config.yaml --phases $(BENCH_PHASES) --output /output/bench-results.json $(BENCH_ARGS)"
	@echo "=== Fetching results..."
	$(SCP) $(VM_USER)@$(VM_IP):$(VM_DIR)/bench-output/bench-results.json bench-results.json
	@echo "=== Results saved to bench-results.json"
	@rm -f bench-arm64

vm-bench-fast: vm-check-env
	$(MAKE) vm-bench BENCH_PHASES=percolator,scoring,db-dedup,db-queries,market

vm-bench-yad2: vm-check-env
	$(MAKE) vm-bench BENCH_PHASES=yad2,cycle BENCH_ARGS="--yad2-cooldown 60s"

vm-bench-results: vm-check-env
	$(SCP) $(VM_USER)@$(VM_IP):$(VM_DIR)/bench-output/bench-results.json bench-results.json
	@cat bench-results.json | python3 -m json.tool 2>/dev/null || cat bench-results.json

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
	$(SSH) 'for c in carwatch-api carwatch-bot-poller carwatch-scraper carwatch-notifier carwatch-enricher; do docker logs --tail 200 -f $$c 2>&1 | sed "s/^/[$$c] /" & done; wait'

LOGS_FILTER ?=
logs: vm-check-env
ifdef LOGS_FILTER
	$(SSH) 'for c in carwatch-api carwatch-bot-poller carwatch-scraper carwatch-notifier carwatch-enricher; do docker logs --tail 500 -f $$c 2>&1 | sed "s/^/[$$c] /" & done; wait' | grep -iF --line-buffered -- "$(LOGS_FILTER)"
else
	$(SSH) 'for c in carwatch-api carwatch-bot-poller carwatch-scraper carwatch-notifier carwatch-enricher; do docker logs --tail 200 -f $$c 2>&1 | sed "s/^/[$$c] /" & done; wait'
endif

vm-status: vm-check-env
	$(SSH) "docker ps --filter name=carwatch && echo '---' && docker exec carwatch /api-server -version"

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

vm-smoke: vm-check-env
	$(SCP) scripts/smoke-test.sh $(VM_USER)@$(VM_IP):$(VM_DIR)/scripts/smoke-test.sh
	$(SSH) "chmod +x $(VM_DIR)/scripts/smoke-test.sh && $(VM_DIR)/scripts/smoke-test.sh $(VM_DIR)"

vm-deploy: vm-sync
	$(SSH) "$(VM_COMPOSE) pull carwatch && $(VM_COMPOSE) up -d --force-recreate carwatch \
		&& sleep 3 && docker exec carwatch /api-server -version"

vm-deploy-all: vm-sync
	$(SSH) "$(VM_COMPOSE) pull && $(VM_COMPOSE) up -d \
		&& sleep 3 && docker exec carwatch /api-server -version"

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
