.PHONY: build run test clean

build:
	go build -o bot ./cmd/bot

run: build
	./bot -config config.yaml

test:
	go test ./...

clean:
	rm -f bot
	rm -rf data/

lint:
	golangci-lint run ./...

docker-build:
	docker build -t car-bot .

docker-run:
	docker compose up -d
