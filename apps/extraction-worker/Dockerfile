######################################################################
# DocLens extraction worker
#
# Multi-stage:
#   1. golang stage compiles the worker binary statically.
#   2. python:3.12-slim runtime layer adds MarkItDown + Tesseract +
#      FFmpeg + Poppler. We pin everything for reproducible builds
#      and a smaller attack surface (per ADR 0012).
######################################################################

ARG GO_VERSION=1.23
ARG PYTHON_VERSION=3.12

# ---- 1. build the Go binary -----------------------------------------
FROM golang:${GO_VERSION}-bookworm AS build

WORKDIR /src

# Copy the workspace files so go.work resolves replaces.
COPY go.work go.work.sum ./
COPY apps/extraction-worker ./apps/extraction-worker
COPY services ./services

# Compile statically so the runtime image doesn't need libc.
ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

RUN cd apps/extraction-worker && \
    go build -trimpath -ldflags '-s -w' \
    -o /out/worker ./cmd/worker

# ---- 2. runtime: Python + MarkItDown + Poppler + Tesseract ----------
FROM python:${PYTHON_VERSION}-slim-bookworm AS runtime

# Pin everything we install. apt versions float per Debian point
# release; we accept that and rely on the digest pin in CI.
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        ca-certificates \
        ffmpeg \
        poppler-utils \
        tesseract-ocr \
        tesseract-ocr-eng \
    && \
    rm -rf /var/lib/apt/lists/*

# Install MarkItDown with all optional dependencies.
# `markitdown[all]` pulls in pdfminer.six, mammoth, openpyxl, etc.
ARG MARKITDOWN_VERSION=0.1.4
RUN pip install --no-cache-dir "markitdown[all]==${MARKITDOWN_VERSION}"

# Run as a non-root user — limits subprocess blast radius if a
# malicious document exploits a parser bug.
RUN useradd --system --create-home --uid 10001 doclens

COPY --from=build /out/worker /usr/local/bin/worker
COPY apps/extraction-worker/Dockerfile.entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

USER doclens
WORKDIR /home/doclens

# Defaults can be overridden at runtime.
ENV GO_ENV=production \
    REDIS_URL=redis://redis:6379/0 \
    DATABASE_URL=postgres://doclens:doclens@postgres:5432/doclens?sslmode=disable \
    S3_ENDPOINT=http://minio:9000 \
    S3_USE_PATH_STYLE=true \
    S3_BUCKET_RAW=doclens-raw \
    S3_BUCKET_ARTIFACTS=doclens-artifacts \
    MARKITDOWN_BIN=markitdown \
    MARKITDOWN_TIMEOUT=5m \
    THUMBNAILER_BIN=pdftoppm \
    EXTRACTION_ENABLED_FORMATS=application/pdf \
    WORKER_CONCURRENCY=4

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
CMD ["worker"]
