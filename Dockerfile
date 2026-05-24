FROM node:22-alpine AS frontend
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ .
ARG VITE_FIREBASE_API_KEY
ARG VITE_FIREBASE_AUTH_DOMAIN
ARG VITE_FIREBASE_PROJECT_ID
ARG VITE_FIREBASE_STORAGE_BUCKET
ARG VITE_FIREBASE_MESSAGING_SENDER_ID
ARG VITE_FIREBASE_APP_ID
RUN npm run build

FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /web/dist ./web/dist
ARG VERSION=dev
ARG GIT_COMMIT=unknown
ARG BUILD_TIME=unknown
ENV LDFLAGS="-s -w -X main.version=${VERSION} -X main.gitCommit=${GIT_COMMIT} -X main.buildTime=${BUILD_TIME}"
RUN CGO_ENABLED=0 go build -ldflags="${LDFLAGS}" -o /bin/api-server ./cmd/api-server && \
    CGO_ENABLED=0 go build -ldflags="${LDFLAGS}" -o /bin/bot-poller ./cmd/bot-poller && \
    CGO_ENABLED=0 go build -ldflags="${LDFLAGS}" -o /bin/scraper ./cmd/scraper && \
    CGO_ENABLED=0 go build -ldflags="${LDFLAGS}" -o /bin/notifier ./cmd/notifier

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -u 1000 carwatch
USER carwatch
COPY --from=builder /bin/api-server /bin/bot-poller /bin/scraper /bin/notifier /usr/local/bin/
COPY --from=builder /app/migrations /migrations
HEALTHCHECK --interval=60s --timeout=5s --retries=3 \
  CMD wget -q --spider http://localhost:8080/healthz || exit 1
ENTRYPOINT ["/usr/local/bin/api-server"]
CMD ["-config", "/config.yaml"]
