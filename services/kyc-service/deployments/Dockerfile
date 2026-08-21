# =============================================================================
# KYC Service Dockerfile
# Multi-stage build for optimized production image
# Supports Go workspace with shared pkg, file-service, user-service,
# vehicle-service and support-service
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
COPY services/support-service/go.mod services/support-service/go.sum* ./services/support-service/
COPY services/email-service/go.mod services/email-service/go.sum* ./services/email-service/
COPY services/kyc-service/go.mod services/kyc-service/go.sum* ./services/kyc-service/

# Stubs de modules internes + go.work en un seul RUN (layer atomique) :
#  - pkg-test       : requis par file-service/go.mod (replace directive)
#  - trips-service  : vehicle-service/go.mod le référence (replace directive) ;
#                     go.mod minimal sans requires — workspace mode utilise le graphe combiné
#  - booking-service, rating-service : requis par trips-service/go.mod mais jamais compilés
# vehicle-service est requis à double titre : dépendance directe de kyc-service
#   (client SetVehicleVerification — is_verified par véhicule) et replace directive
#   de file-service/go.mod.
# support-service est requis car kyc-service importe support-service/proto/gen.
# email-service est requis car support-service/go.mod y a un replace directive.
# Tout dans un seul layer : si le contenu change, go.work est aussi invalidé (pas de cache stale).
RUN mkdir -p pkg-test \
             services/trips-service/proto/gen \
             services/booking-service \
             services/rating-service && \
    printf 'module github.com/Kpeewu/tissi-mah/pkg-test\n\ngo 1.25\n' \
        > pkg-test/go.mod && \
    printf 'module github.com/Kpeewu/tissi-mah/services/trips-service\n\ngo 1.25\n' \
        > services/trips-service/go.mod && \
    printf 'module github.com/Kpeewu/tissi-mah/services/booking-service\n\ngo 1.25\n' \
        > services/booking-service/go.mod && \
    printf 'module github.com/Kpeewu/tissi-mah/services/rating-service\n\ngo 1.25\n' \
        > services/rating-service/go.mod && \
    printf 'go 1.25.6\n\nuse (\n\t./pkg\n\t./pkg-test\n\t./services/booking-service\n\t./services/email-service\n\t./services/file-service\n\t./services/kyc-service\n\t./services/rating-service\n\t./services/support-service\n\t./services/trips-service\n\t./services/user-service\n\t./services/vehicle-service\n)\n' \
        > go.work

# Download dependencies for all modules
RUN go mod download -C ./pkg
RUN go mod download -C ./services/file-service
RUN go mod download -C ./services/user-service
RUN go mod download -C ./services/vehicle-service
RUN go mod download -C ./services/support-service
RUN go mod download -C ./services/email-service
RUN go mod download -C ./services/kyc-service

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

# Copy file-service source code (required by kyc-service replace directive)
COPY services/file-service/ ./services/file-service/

# Copy user-service source code (required by file-service replace directive)
COPY services/user-service/ ./services/user-service/

# Copy vehicle-service source code (kyc-service client + file-service replace directive)
COPY services/vehicle-service/ ./services/vehicle-service/

# Copy support-service source code (required by kyc-service replace directive)
COPY services/support-service/ ./services/support-service/

# Copy email-service source code (required by support-service replace directive)
COPY services/email-service/ ./services/email-service/

# Copy kyc-service source code
COPY services/kyc-service/ ./services/kyc-service/

# Build from the service directory using workspace
WORKDIR /build/services/kyc-service

# Build with optimizations
RUN BUILD_TIME=${BUILD_TIME:-$(date -u +%Y-%m-%dT%H:%M:%SZ)} && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s \
        -X main.version=${VERSION} \
        -X main.commitSHA=${COMMIT_SHA} \
        -X main.buildTime=${BUILD_TIME}" \
    -trimpath \
    -o /build/kyc-service \
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
LABEL org.opencontainers.image.title="kyc-service" \
      org.opencontainers.image.description="tissiMah KYC Service" \
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
COPY --from=builder --chown=appuser:appuser /build/kyc-service .

# Copy grpc_health_probe for Kubernetes TLS-enabled health checks
COPY --from=ghcr.io/grpc-ecosystem/grpc-health-probe:v0.4.24 /ko-app/grpc-health-probe /usr/local/bin/grpc_health_probe

# Switch to non-root user
USER appuser

# Expose gRPC port and metrics port
EXPOSE 50057 9090

# Health check via le protocole gRPC health (grpc.health.v1.Health).
# Mêmes options que les probes Kubernetes : TLS intra-cluster, certificat non vérifié.
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD grpc_health_probe -addr=:${GRPC_PORT} -tls -tls-no-verify || exit 1

# Default environment variables
ENV ENVIRONMENT=production \
    LOG_LEVEL=info \
    GRPC_PORT=50057 \
    METRICS_PORT=9090

# Run the service
ENTRYPOINT ["/app/kyc-service"]
CMD ["serve"]
