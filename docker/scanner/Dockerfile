# syntax=docker/dockerfile:1.7

FROM golang:1.25.5-alpine AS builder
WORKDIR /src

RUN apk add --no-cache ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/apant-scanner ./cmd/scanner

FROM debian:stable-slim
WORKDIR /app

RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates tzdata nmap libcap2-bin && \
    setcap cap_net_raw,cap_net_admin+eip /usr/bin/nmap && \
    apt-get clean && rm -rf /var/lib/apt/lists/* && \
    useradd -m -s /usr/sbin/nologin scanner

COPY --from=builder /out/apant-scanner /app/apant-scanner

ENV SCANNER_PORT=8081
ENV NMAP_BINARY=nmap
EXPOSE 8081

USER scanner

CMD ["/app/apant-scanner"]
