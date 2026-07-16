# syntax=docker/dockerfile:1.7

FROM golang:1.25.7-bookworm AS builder
WORKDIR /src

ARG SCANNER_UID=10001
ARG SCANNER_GID=10001

RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates tzdata git build-essential && \
    apt-get clean && rm -rf /var/lib/apt/lists/*

# Install third-party scanner tools BEFORE copying app source, so changing Go code
# does not invalidate this slow, network-heavy layer (it stays cached).
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=1 GOBIN=/out/tools go install github.com/ffuf/ffuf/v2@latest && \
    CGO_ENABLED=1 GOBIN=/out/tools go install github.com/lc/gau/v2/cmd/gau@latest && \
    CGO_ENABLED=1 GOBIN=/out/tools go install github.com/tomnomnom/waybackurls@latest && \
    CGO_ENABLED=1 GOBIN=/out/tools go install github.com/projectdiscovery/httpx/cmd/httpx@latest && \
    CGO_ENABLED=1 GOBIN=/out/tools go install github.com/projectdiscovery/subfinder/v2/cmd/subfinder@latest && \
    CGO_ENABLED=1 GOBIN=/out/tools go install github.com/hahwul/dalfox/v2@latest && \
    CGO_ENABLED=1 GOBIN=/out/tools go install github.com/projectdiscovery/nuclei/v3/cmd/nuclei@latest && \
    CGO_ENABLED=1 GOBIN=/out/tools go install github.com/projectdiscovery/katana/cmd/katana@latest && \
    CGO_ENABLED=0 GOBIN=/out/tools go install github.com/zricethezav/gitleaks/v8@latest && \
    CGO_ENABLED=0 GOBIN=/out/tools go install github.com/google/osv-scanner/cmd/osv-scanner@v1.9.2

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/apant-scanner ./cmd/scanner

FROM debian:stable-slim
WORKDIR /app

ARG SCANNER_UID=10001
ARG SCANNER_GID=10001

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    ca-certificates \
    tzdata \
    nmap \
    libcap2-bin \
    python3 \
    python3-venv \
    ripgrep \
    curl \
    jq \
    git \
    && setcap cap_net_raw,cap_net_admin+eip /usr/bin/nmap \
    && python3 -m venv /opt/mitmproxy-venv \
    && /opt/mitmproxy-venv/bin/pip install --no-cache-dir --retries 10 --timeout 300 mitmproxy \
    && ln -s /opt/mitmproxy-venv/bin/mitmproxy /usr/local/bin/mitmproxy \
    && ln -s /opt/mitmproxy-venv/bin/mitmdump /usr/local/bin/mitmdump \
    && ln -s /opt/mitmproxy-venv/bin/mitmweb /usr/local/bin/mitmweb \
    && python3 -m venv /opt/semgrep-venv \
    && /opt/semgrep-venv/bin/pip install --no-cache-dir --retries 10 --timeout 300 semgrep \
    && ln -s /opt/semgrep-venv/bin/semgrep /usr/local/bin/semgrep \
    && git clone --depth 1 https://github.com/semgrep/semgrep-rules.git /opt/semgrep-rules \
    && rm -rf /opt/semgrep-rules/.git \
    && git clone --depth 1 https://github.com/sqlmapproject/sqlmap.git /opt/sqlmap \
    && ln -s /opt/sqlmap/sqlmap.py /usr/local/bin/sqlmap \
    && chmod +x /opt/sqlmap/sqlmap.py \
    && ln -s /usr/bin/python3 /usr/local/bin/python \
    && mkdir -p /wordlists \
    && curl -fsSL https://raw.githubusercontent.com/danielmiessler/SecLists/master/Discovery/Web-Content/common.txt \
    -o /wordlists/common.txt \
    && apt-get clean && rm -rf /var/lib/apt/lists/* \
    && groupadd -g ${SCANNER_GID} scanner \
    && useradd -m -u ${SCANNER_UID} -g ${SCANNER_GID} -s /usr/sbin/nologin scanner \
    && mkdir -p \
    /home/scanner/.cache \
    /home/scanner/.config \
    /home/scanner/.sqlmap \
    /home/scanner/.local \
    /home/scanner/.nuclei-templates \
    /home/scanner/nuclei-reports \
    && chown -R scanner:scanner /home/scanner

COPY --from=builder /out/apant-scanner /app/apant-scanner
COPY --from=builder /out/tools/ /usr/local/bin/

# Curated APANT SAST rules (PHP/JS) layered on top of the OSS semgrep rules.
COPY docker/scanner/apant-semgrep-rules/ /opt/apant-rules/

# Wrapper nuclei to force HOME and write templates config each run.
COPY docker/scanner/nuclei-wrapper.sh /usr/local/bin/nuclei-wrapper
RUN sed -i 's/\r$//' /usr/local/bin/nuclei-wrapper \
    && chmod +x /usr/local/bin/nuclei-wrapper \
    && mv /usr/local/bin/nuclei /usr/local/bin/nuclei-real \
    && mv /usr/local/bin/nuclei-wrapper /usr/local/bin/nuclei

# Ensure templates resolve under the scanner home.
RUN rm -f /app/nuclei-templates && \
    ln -s /home/scanner/.nuclei-templates /home/scanner/nuclei-templates

ENV SCANNER_PORT=8081
ENV NMAP_BINARY=nmap
ENV NUCLEI_TEMPLATES_PATH=/home/scanner/.nuclei-templates
ENV HOME=/home/scanner
# Point at the curated APANT pack plus the OSS rules' per-language subdirs. The
# OSS repo ROOT must not be used as a config: it contains non-rule yaml (e.g.
# .pre-commit-config.yaml) that makes semgrep abort the whole run. Point at the
# per-language subdirs only. python/java/go/ruby carry real taint-mode (data-flow
# source→sink) rules, so the deterministic detector covers those languages without
# relying on the PHP/JS-oriented regex discovery greps.
ENV SEMGREP_CONFIG="/opt/apant-rules /opt/semgrep-rules/php /opt/semgrep-rules/javascript /opt/semgrep-rules/generic /opt/semgrep-rules/python /opt/semgrep-rules/java /opt/semgrep-rules/go /opt/semgrep-rules/ruby"
ENV SCANNER_WORKSPACE_DIR=/workspace
EXPOSE 8081

USER scanner

CMD ["/app/apant-scanner"]
