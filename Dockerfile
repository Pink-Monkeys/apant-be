# syntax=docker/dockerfile:1.7

FROM golang:1.25.7-alpine AS builder
WORKDIR /src

RUN apk add --no-cache ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/apant-be ./cmd/api && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate

FROM alpine:3.22
WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata su-exec && \
    addgroup -S app && adduser -S -G app app && \
    mkdir -p /workspace && chown app:app /workspace

COPY --from=builder /out/apant-be /app/apant-be
COPY --from=builder /out/migrate /app/migrate
COPY docker/api-entrypoint.sh /usr/local/bin/api-entrypoint
RUN sed -i 's/\r$//' /usr/local/bin/api-entrypoint && chmod +x /usr/local/bin/api-entrypoint

ENV PORT=8000
EXPOSE 8000

# Starts as root only to fix the shared /workspace volume ownership, then the
# entrypoint drops to the unprivileged "app" user before running the binary.
ENTRYPOINT ["/usr/local/bin/api-entrypoint"]
CMD ["/app/apant-be"]
