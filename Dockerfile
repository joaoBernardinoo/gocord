# syntax=docker/dockerfile:1
FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

# Mount both the Go module cache and the Go build cache
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /app/ipv6-video-call ./cmd/server

FROM alpine:3.20

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

RUN adduser -D -u 10001 appuser
USER appuser

COPY --from=builder /app/ipv6-video-call /app/ipv6-video-call

EXPOSE 8080

ENTRYPOINT ["/app/ipv6-video-call"]
