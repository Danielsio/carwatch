FROM golang:1.24-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o /bot ./cmd/bot

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -u 1000 bot
USER bot
COPY --from=builder /bot /bot
VOLUME /data
ENTRYPOINT ["/bot"]
CMD ["-config", "/config.yaml"]
