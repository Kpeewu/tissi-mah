# =============================================================================
# Vehicle Service Dockerfile
# Multi-stage build for optimized production image
# Supports Go workspace with shared pkg module and file-service dependency
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
COPY services/file-service/go.mod services/file-service/go.sum* ./services/file-service/
COPY services/vehicle-service/go.mod services/vehicle-service/go.sum* ./services/vehicle-service/

# pkg-test est un module de test uniquement, mais file-service/go.mod a un replace directive qui
# pointe vers lui. On crée un stub minimal pour satisfaire la résolution Go sans inclure
# aucune dépendance de test dans l'image de production.
RUN mkdir -p pkg-test && \
    printf 'module github.com/Kpeewu/tissi-mah/pkg-test\n\ngo 1.25\n' > pkg-test/go.mod

# Create a scoped go.work with pkg + pkg-test + file-service + vehicle-service
RUN printf 'go 1.25.6\n\nuse (\n\t./pkg\n\t./pkg-test\n\t./services/file-service\n\t./services/vehicle-service\n)\n' > go.work

# Download dependencies for all modules
RUN go mod download -C ./pkg
RUN go mod download -C ./services/file-service
RUN go mod download -C ./services/vehicle-service

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

# Copy file-service source code (required by vehicle-service replace directive)
COPY services/file-service/ ./services/file-service/

# Copy vehicle-service source code
COPY services/vehicle-service/ ./services/vehicle-service/

# Build from the service directory using workspace
WORKDIR /build/services/vehicle-service

# Build with optimizations
RUN BUILD_TIME=${BUILD_TIME:-$(date -u +%Y-%m-%dT%H:%M:%SZ)} && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s \
        -X main.version=${VERSION} \
        -X main.commitSHA=${COMMIT_SHA} \
        -X main.buildTime=${BUILD_TIME}" \
    -trimpath \
    -o /build/vehicle-service \
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
LABEL org.opencontainers.image.title="vehicle-service" \
      org.opencontainers.image.description="tissiMah Vehicle Service" \
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
COPY --from=builder --chown=appuser:appuser /build/vehicle-service .

# Copy migrations if they exist
COPY --from=builder --chown=appuser:appuser /build/services/vehicle-service/migrations ./migrations/

# Switch to non-root user
# Copy golang-migrate binary for Kubernetes initContainer
COPY --from=migrate/migrate:v4.17.0 /usr/local/bin/migrate /usr/local/bin/migrate

USER appuser

# Expose gRPC port and metrics port
EXPOSE 50055 9090

# Health check using grpc health protocol
# Note: This requires the service to implement grpc.health.v1.Health
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["/app/vehicle-service", "health"] || exit 1

# Default environment variables
ENV ENVIRONMENT=production \
    LOG_LEVEL=info \
    GRPC_PORT=50055 \
    METRICS_PORT=9090

# Run the service
ENTRYPOINT ["/app/vehicle-service"]
CMD ["serve"]
