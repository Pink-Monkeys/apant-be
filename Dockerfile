# syntax=docker/dockerfile:1.7

FROM golang:1.25.5-alpine AS builder
WORKDIR /src

RUN apk add --no-cache ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/apant-be ./cmd/api

FROM alpine:3.22
WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata docker-cli && \
    addgroup -S app && adduser -S -G app app

COPY --from=builder /out/apant-be /app/apant-be

ENV PORT=8080
ENV DOCKER_BINARY=docker
EXPOSE 8080

USER app

CMD ["/app/apant-be"]
