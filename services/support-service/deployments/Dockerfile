# =============================================================================
# Support Service Dockerfile
# Multi-stage build for optimized production image
# =============================================================================

ARG GO_VERSION=1.25
ARG ALPINE_VERSION=3.21

# =============================================================================
# Stage 1: Dependencies
# =============================================================================
FROM golang:${GO_VERSION}-alpine AS deps

WORKDIR /build

RUN apk add --no-cache git ca-certificates tzdata

# Copy go.mod files first (better caching)
COPY pkg/go.mod pkg/go.sum ./pkg/
COPY services/email-service/go.mod services/email-service/go.sum* ./services/email-service/
COPY services/support-service/go.mod services/support-service/go.sum* ./services/support-service/

# Scoped go.work limited to modules present in this build context
RUN printf 'go 1.25.6\n\nuse (\n\t./pkg\n\t./services/email-service\n\t./services/support-service\n)\n' > go.work

RUN go mod download -C ./pkg
RUN go mod download -C ./services/email-service
RUN go mod download -C ./services/support-service

# =============================================================================
# Stage 2: Builder
# =============================================================================
FROM deps AS builder

ARG VERSION=dev
ARG COMMIT_SHA=unknown
ARG BUILD_TIME

WORKDIR /build

COPY pkg/ ./pkg/
COPY services/email-service/ ./services/email-service/
COPY services/support-service/ ./services/support-service/

WORKDIR /build/services/support-service

RUN BUILD_TIME=${BUILD_TIME:-$(date -u +%Y-%m-%dT%H:%M:%SZ)} && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s \
        -X main.version=${VERSION} \
        -X main.commitSHA=${COMMIT_SHA} \
        -X main.buildTime=${BUILD_TIME}" \
    -trimpath \
    -o /build/support-service \
    ./cmd/server/main.go

# =============================================================================
# Stage 3: Runtime
# =============================================================================
FROM alpine:${ALPINE_VERSION} AS runtime

ARG VERSION=dev
ARG COMMIT_SHA=unknown
ARG BUILD_TIME

LABEL org.opencontainers.image.title="support-service" \
      org.opencontainers.image.description="tissiMah Support / Admin Back-Office Service" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT_SHA}" \
      org.opencontainers.image.created="${BUILD_TIME}" \
      org.opencontainers.image.source="https://github.com/Kpeewu/tissi-mah" \
      org.opencontainers.image.vendor="tissiMah" \
      org.opencontainers.image.licenses="MIT"

RUN apk --no-cache add \
    ca-certificates \
    tzdata \
    && update-ca-certificates \
    && rm -rf /var/cache/apk/*

RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser -h /app -s /sbin/nologin appuser

WORKDIR /app

COPY --from=builder --chown=appuser:appuser /build/support-service .
COPY --from=builder --chown=appuser:appuser /build/services/support-service/migrations ./migrations/

# Copy golang-migrate binary for Kubernetes initContainer
COPY --from=migrate/migrate:v4.17.0 /usr/local/bin/migrate /usr/local/bin/migrate

# Copy grpc_health_probe for Kubernetes TLS-enabled health checks
COPY --from=ghcr.io/grpc-ecosystem/grpc-health-probe:v0.4.24 /ko-app/grpc-health-probe /usr/local/bin/grpc_health_probe

USER appuser

EXPOSE 50063

ENV ENVIRONMENT=production \
    LOG_LEVEL=info \
    GRPC_PORT=50063

ENTRYPOINT ["/app/support-service"]
