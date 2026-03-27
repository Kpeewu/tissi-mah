# =============================================================================
# Booking Service Dockerfile
# Multi-stage build for optimized production image
# Supports Go workspace with shared pkg, user-service, trips-service,
# vehicle-service (transitive via trips-service), file-service (transitive via vehicle-service)
# and booking-service
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
COPY services/user-service/go.mod services/user-service/go.sum* ./services/user-service/
COPY services/vehicle-service/go.mod services/vehicle-service/go.sum* ./services/vehicle-service/
COPY services/trips-service/go.mod services/trips-service/go.sum* ./services/trips-service/
COPY services/booking-service/go.mod services/booking-service/go.sum* ./services/booking-service/
COPY services/payment-service/go.mod services/payment-service/go.sum* ./services/payment-service/

# pkg-test est un module de test uniquement, mais plusieurs go.mod ont un replace directive qui
# pointe vers lui. On crée un stub minimal (go.mod vide) pour satisfaire la résolution Go sans
# inclure aucune dépendance de test dans l'image de production.
RUN mkdir -p pkg-test && \
    printf 'module github.com/Kpeewu/tissi-mah/pkg-test\n\ngo 1.25\n' > pkg-test/go.mod

# Create a scoped go.work with all required modules
# file-service is required because vehicle-service has a replace directive pointing to it,
# vehicle-service is required because trips-service has a replace directive pointing to it
# payment-service is required because booking-service has a replace directive pointing to it
RUN printf 'go 1.25.6\n\nuse (\n\t./pkg\n\t./pkg-test\n\t./services/file-service\n\t./services/user-service\n\t./services/vehicle-service\n\t./services/trips-service\n\t./services/booking-service\n\t./services/payment-service\n)\n' > go.work

# Download dependencies for all modules
RUN go mod download -C ./pkg
RUN go mod download -C ./services/file-service
RUN go mod download -C ./services/user-service
RUN go mod download -C ./services/vehicle-service
RUN go mod download -C ./services/trips-service
RUN go mod download -C ./services/booking-service
RUN go mod download -C ./services/payment-service

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

# Copy user-service source code (required by booking-service + trips-service replace directive)
COPY services/user-service/ ./services/user-service/

# Copy vehicle-service source code (required by trips-service replace directive)
COPY services/vehicle-service/ ./services/vehicle-service/

# Copy trips-service source code (required by booking-service replace directive)
COPY services/trips-service/ ./services/trips-service/

# Copy booking-service source code
COPY services/booking-service/ ./services/booking-service/

# Copy payment-service source code (required by booking-service replace directive)
COPY services/payment-service/ ./services/payment-service/

# Build from the service directory using workspace
WORKDIR /build/services/booking-service

# Build with optimizations
RUN BUILD_TIME=${BUILD_TIME:-$(date -u +%Y-%m-%dT%H:%M:%SZ)} && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s \
        -X main.version=${VERSION} \
        -X main.commitSHA=${COMMIT_SHA} \
        -X main.buildTime=${BUILD_TIME}" \
    -trimpath \
    -o /build/booking-service \
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
LABEL org.opencontainers.image.title="booking-service" \
      org.opencontainers.image.description="tissiMah Booking Service" \
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
COPY --from=builder --chown=appuser:appuser /build/booking-service .

# Copy migrations if they exist
COPY --from=builder --chown=appuser:appuser /build/services/booking-service/migrations ./migrations/

# Switch to non-root user
USER appuser

# Expose gRPC port and metrics port
EXPOSE 50058 9090

# Health check using grpc health protocol
# Note: This requires the service to implement grpc.health.v1.Health
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["/app/booking-service", "health"] || exit 1

# Default environment variables
ENV ENVIRONMENT=production \
    LOG_LEVEL=info \
    GRPC_PORT=50058 \
    METRICS_PORT=9090

# Run the service
ENTRYPOINT ["/app/booking-service"]
CMD ["serve"]
