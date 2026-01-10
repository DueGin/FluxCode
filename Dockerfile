# =============================================================================
# FluxCode Multi-Stage Dockerfile
# =============================================================================
# Stage 1: Build frontend
# Stage 2: Build Go backend with embedded frontend
# Stage 3: Final minimal image
# =============================================================================

ARG NODE_IMAGE=node:24-alpine
ARG GOLANG_IMAGE=golang:1.25-alpine
ARG ALPINE_IMAGE=alpine:3.19
ARG GOPROXY=https://goproxy.cn,direct
ARG GOSUMDB=sum.golang.google.cn
# Alpine APK 镜像（多架构构建时偶发网络问题可通过切换镜像缓解）
# - 默认使用国内镜像加速（Aliyun），失败时回退官方源
ARG APK_MIRROR=https://mirrors.aliyun.com/alpine
ARG APK_MIRROR_FALLBACK=https://dl-cdn.alpinelinux.org/alpine

# -----------------------------------------------------------------------------
# Stage 1: Frontend Builder
# -----------------------------------------------------------------------------
FROM ${NODE_IMAGE} AS frontend-builder

WORKDIR /app/frontend

# Install dependencies first (better caching)
COPY frontend/package*.json ./
RUN npm ci

# Copy frontend source and build
COPY frontend/ ./
RUN npm run build

# -----------------------------------------------------------------------------
# Stage 2: Backend Builder
# -----------------------------------------------------------------------------
FROM ${GOLANG_IMAGE} AS backend-builder

# Build arguments for version info (set by CI)
ARG VERSION=docker
ARG COMMIT=docker
ARG DATE
ARG GOPROXY
ARG GOSUMDB
ARG APK_MIRROR
ARG APK_MIRROR_FALLBACK

ENV GOPROXY=${GOPROXY}
ENV GOSUMDB=${GOSUMDB}

# Install build dependencies
RUN set -eux; \
    APK_MIRROR="${APK_MIRROR%/}"; \
    APK_MIRROR_FALLBACK="${APK_MIRROR_FALLBACK%/}"; \
    sed -i \
      -e "s|https://dl-cdn.alpinelinux.org/alpine|${APK_MIRROR}|g" \
      -e "s|http://dl-cdn.alpinelinux.org/alpine|${APK_MIRROR}|g" \
      /etc/apk/repositories; \
    ok=0; \
    for i in 1 2 3; do \
      if apk add --no-cache git ca-certificates tzdata; then ok=1; break; fi; \
      echo "apk add failed (attempt ${i}), retrying..." >&2; \
      sleep $((i * 2)); \
    done; \
    if [ "$ok" -ne 1 ]; then \
      echo "apk add still failed, switching mirror to fallback: ${APK_MIRROR_FALLBACK}" >&2; \
      sed -i \
        -e "s|${APK_MIRROR}|${APK_MIRROR_FALLBACK}|g" \
        -e "s|https://dl-cdn.alpinelinux.org/alpine|${APK_MIRROR_FALLBACK}|g" \
        -e "s|http://dl-cdn.alpinelinux.org/alpine|${APK_MIRROR_FALLBACK}|g" \
        /etc/apk/repositories; \
      for i in 1 2 3; do \
        if apk add --no-cache git ca-certificates tzdata; then ok=1; break; fi; \
        echo "apk add (fallback) failed (attempt ${i}), retrying..." >&2; \
        sleep $((i * 2)); \
      done; \
    fi; \
    [ "$ok" -eq 1 ]

WORKDIR /app/backend

# Copy go mod files first (better caching)
COPY backend/go.mod backend/go.sum ./
RUN go mod download

# Copy backend source first
COPY backend/ ./

# Copy frontend dist from previous stage (must be after backend copy to avoid being overwritten)
COPY --from=frontend-builder /app/backend/internal/web/dist ./internal/web/dist

# Build the binary (BuildType=release for CI builds, embed frontend)
RUN CGO_ENABLED=0 GOOS=linux go build \
    -tags embed \
    -ldflags="-s -w -X main.Commit=${COMMIT} -X main.Date=${DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)} -X main.BuildType=release" \
    -o /app/fluxcode \
    ./cmd/server

# -----------------------------------------------------------------------------
# Stage 3: Final Runtime Image
# -----------------------------------------------------------------------------
FROM ${ALPINE_IMAGE}

# Labels
LABEL maintainer="DueGin <github.com/DueGin>"
LABEL description="FluxCode - AI API Gateway Platform"
LABEL org.opencontainers.image.source="https://github.com/DueGin/FluxCode"

# Re-declare build args for this stage
ARG APK_MIRROR
ARG APK_MIRROR_FALLBACK

# Install runtime dependencies
RUN set -eux; \
    APK_MIRROR="${APK_MIRROR%/}"; \
    APK_MIRROR_FALLBACK="${APK_MIRROR_FALLBACK%/}"; \
    sed -i \
      -e "s|https://dl-cdn.alpinelinux.org/alpine|${APK_MIRROR}|g" \
      -e "s|http://dl-cdn.alpinelinux.org/alpine|${APK_MIRROR}|g" \
      /etc/apk/repositories; \
    ok=0; \
    for i in 1 2 3; do \
      if apk add --no-cache ca-certificates tzdata curl; then ok=1; break; fi; \
      echo "apk add failed (attempt ${i}), retrying..." >&2; \
      sleep $((i * 2)); \
    done; \
    if [ "$ok" -ne 1 ]; then \
      echo "apk add still failed, switching mirror to fallback: ${APK_MIRROR_FALLBACK}" >&2; \
      sed -i \
        -e "s|${APK_MIRROR}|${APK_MIRROR_FALLBACK}|g" \
        -e "s|https://dl-cdn.alpinelinux.org/alpine|${APK_MIRROR_FALLBACK}|g" \
        -e "s|http://dl-cdn.alpinelinux.org/alpine|${APK_MIRROR_FALLBACK}|g" \
        /etc/apk/repositories; \
      for i in 1 2 3; do \
        if apk add --no-cache ca-certificates tzdata curl; then ok=1; break; fi; \
        echo "apk add (fallback) failed (attempt ${i}), retrying..." >&2; \
        sleep $((i * 2)); \
      done; \
    fi; \
    [ "$ok" -eq 1 ]; \
    rm -rf /var/cache/apk/*

# Create non-root user
RUN addgroup -g 1000 fluxcode && \
    adduser -u 1000 -G fluxcode -s /bin/sh -D fluxcode

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=backend-builder /app/fluxcode /app/fluxcode

# Create data directory
RUN mkdir -p /app/data && chown -R fluxcode:fluxcode /app

# Switch to non-root user
USER fluxcode

# Expose port (can be overridden by SERVER_PORT env var)
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:${SERVER_PORT:-8080}/health || exit 1

# Run the application
ENTRYPOINT ["/app/fluxcode"]
