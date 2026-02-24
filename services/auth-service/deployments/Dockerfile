# =============================================================================
# Auth Service Dockerfile
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

# Copy workspace file and all go.mod files first (for better caching)
COPY go.work go.work.sum* ./
COPY pkg/go.mod pkg/go.sum ./pkg/
COPY pkg-test/go.mod pkg-test/go.sum* ./pkg-test/
COPY services/auth-service/go.mod services/auth-service/go.sum* ./services/auth-service/

# Download dependencies for all modules
RUN GOWORK=off go mod download -C ./pkg
RUN GOWORK=off go mod download -C ./services/auth-service

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

# Copy auth-service source code
COPY services/auth-service/ ./services/auth-service/

# Build from the service directory using workspace
WORKDIR /build/services/auth-service

# Build with optimizations
RUN BUILD_TIME=${BUILD_TIME:-$(date -u +%Y-%m-%dT%H:%M:%SZ)} && \
    GOWORK=off CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s \
        -X main.version=${VERSION} \
        -X main.commitSHA=${COMMIT_SHA} \
        -X main.buildTime=${BUILD_TIME}" \
    -trimpath \
    -o /build/auth-service \
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
LABEL org.opencontainers.image.title="auth-service" \
      org.opencontainers.image.description="tissiMah Authentication Service" \
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
COPY --from=builder --chown=appuser:appuser /build/auth-service .

# Copy migrations if they exist
COPY --from=builder --chown=appuser:appuser /build/services/auth-service/migrations ./migrations/

# Switch to non-root user
USER appuser

# Expose gRPC port and metrics port
EXPOSE 50051 9090

# Health check using grpc health protocol
# Note: This requires the service to implement grpc.health.v1.Health
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["/app/auth-service", "health"] || exit 1

# Default environment variables
ENV ENVIRONMENT=production \
    LOG_LEVEL=info \
    GRPC_PORT=50051 \
    METRICS_PORT=9090

# Run the service
ENTRYPOINT ["/app/auth-service"]
CMD ["serve"]