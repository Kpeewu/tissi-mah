# =============================================================================
# API Gateway Dockerfile
# Multi-stage build for optimized production image
# Supports Go workspace with shared pkg module
# =============================================================================

# Build arguments
ARG GO_VERSION=1.25
ARG ALPINE_VERSION=3.21

# =============================================================================
# Stage 1: Dependencies
# =============================================================================
FROM golang:${GO_VERSION}-alpine AS deps

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Copy go.mod files first (for better caching)
COPY pkg/go.mod pkg/go.sum ./pkg/
COPY services/api-gateway/go.mod services/api-gateway/go.sum* ./services/api-gateway/
# Proto imports need these modules in workspace
COPY services/auth-service/go.mod services/auth-service/go.sum* ./services/auth-service/
COPY services/user-service/go.mod services/user-service/go.sum* ./services/user-service/

# Create a scoped go.work with only the modules present in this build context
# (the repo go.work lists all services, but we only need the ones api-gateway depends on)
RUN printf 'go 1.25.6\n\nuse (\n\t./pkg\n\t./services/api-gateway\n\t./services/auth-service\n\t./services/user-service\n)\n' > go.work

# Download dependencies for all modules
RUN go mod download -C ./pkg
RUN go mod download -C ./services/api-gateway

# =============================================================================
# Stage 2: Builder
# =============================================================================
FROM deps AS builder

# Build arguments for version info
ARG VERSION=dev
ARG COMMIT_SHA=unknown
ARG BUILD_TIME

WORKDIR /build

# Copy shared pkg source code
COPY pkg/ ./pkg/

# Copy api-gateway source code
COPY services/api-gateway/ ./services/api-gateway/

# Copy auth-service and user-service (needed for workspace proto imports)
COPY services/auth-service/ ./services/auth-service/
COPY services/user-service/ ./services/user-service/

# Build from the service directory using workspace
WORKDIR /build/services/api-gateway

# Build with optimizations
RUN BUILD_TIME=${BUILD_TIME:-$(date -u +%Y-%m-%dT%H:%M:%SZ)} && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s \
        -X main.version=${VERSION} \
        -X main.commitSHA=${COMMIT_SHA} \
        -X main.buildTime=${BUILD_TIME}" \
    -trimpath \
    -o /build/api-gateway \
    ./cmd/server/main.go

# =============================================================================
# Stage 3: Runtime
# =============================================================================
FROM alpine:${ALPINE_VERSION} AS runtime

# Build arguments for labels
ARG VERSION=dev
ARG COMMIT_SHA=unknown
ARG BUILD_TIME

# OCI Labels
LABEL org.opencontainers.image.title="api-gateway" \
      org.opencontainers.image.description="tissiMah API Gateway" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT_SHA}" \
      org.opencontainers.image.created="${BUILD_TIME}" \
      org.opencontainers.image.source="https://github.com/Kpeewu/tissi-mah" \
      org.opencontainers.image.vendor="tissiMah" \
      org.opencontainers.image.licenses="MIT"

# Install runtime dependencies
RUN apk --no-cache add \
    ca-certificates \
    tzdata \
    && update-ca-certificates \
    && rm -rf /var/cache/apk/*

# Create non-root user for security
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser -h /app -s /sbin/nologin appuser

WORKDIR /app

# Copy binary from builder
COPY --from=builder --chown=appuser:appuser /build/api-gateway .

# Switch to non-root user
USER appuser

# Expose HTTP port
EXPOSE 8080

# Health check via HTTP GET /health
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Default environment variables
ENV ENVIRONMENT=production \
    LOG_LEVEL=info \
    HTTP_PORT=8080

# Run the service
ENTRYPOINT ["/app/api-gateway"]
