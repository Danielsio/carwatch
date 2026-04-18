.PHONY: build run test lint clean ci docker-build docker-run

build:
	go build -o bot ./cmd/bot

run: build
	./bot -config config.yaml

test:
	go test ./...

lint:
	golangci-lint run ./...

ci: lint test

clean:
	rm -f bot

docker-build:
	docker build -t carwatch .

docker-run:
	docker compose up -d
