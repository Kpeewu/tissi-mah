# =============================================================================
# Rating Service Dockerfile
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
COPY services/rating-service/go.mod services/rating-service/go.sum* ./services/rating-service/

# Create a scoped go.work with only the modules present in this build context
# (the repo go.work lists all services, but we only have pkg + rating-service here)
RUN printf 'go 1.25.6\n\nuse (\n\t./pkg\n\t./services/rating-service\n)\n' > go.work

# Download dependencies for all modules
RUN go mod download -C ./pkg
RUN go mod download -C ./services/rating-service

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

# Copy rating-service source code
COPY services/rating-service/ ./services/rating-service/

# Build from the service directory using workspace
WORKDIR /build/services/rating-service

# Build with optimizations
RUN BUILD_TIME=${BUILD_TIME:-$(date -u +%Y-%m-%dT%H:%M:%SZ)} && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s \
        -X main.version=${VERSION} \
        -X main.commitSHA=${COMMIT_SHA} \
        -X main.buildTime=${BUILD_TIME}" \
    -trimpath \
    -o /build/rating-service \
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
LABEL org.opencontainers.image.title="rating-service" \
      org.opencontainers.image.description="tissiMah Rating Service" \
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
COPY --from=builder --chown=appuser:appuser /build/rating-service .

# Copy migrations if they exist
COPY --from=builder --chown=appuser:appuser /build/services/rating-service/migrations ./migrations/

# Switch to non-root user
# Copy golang-migrate binary for Kubernetes initContainer
COPY --from=migrate/migrate:v4.17.0 /usr/local/bin/migrate /usr/local/bin/migrate

# Copy grpc_health_probe for Kubernetes TLS-enabled health checks
COPY --from=ghcr.io/grpc-ecosystem/grpc-health-probe:v0.4.24 /ko-app/grpc-health-probe /usr/local/bin/grpc_health_probe

USER appuser

# Expose gRPC port and metrics port
EXPOSE 50054 9090

# Health check via le protocole gRPC health (grpc.health.v1.Health).
# Mêmes options que les probes Kubernetes : TLS intra-cluster, certificat non vérifié.
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD grpc_health_probe -addr=:${GRPC_PORT} -tls -tls-no-verify || exit 1

# Default environment variables
ENV ENVIRONMENT=production \
    LOG_LEVEL=info \
    GRPC_PORT=50054 \
    METRICS_PORT=9090

# Run the service
ENTRYPOINT ["/app/rating-service"]
CMD ["serve"]
