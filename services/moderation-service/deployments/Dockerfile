ARG GO_VERSION=1.25
ARG ALPINE_VERSION=3.21

# =============================================================================
# Stage 1: Dependencies
# =============================================================================
FROM golang:${GO_VERSION}-alpine AS deps

WORKDIR /build

RUN apk add --no-cache git ca-certificates tzdata

# Seules dépendances : pkg partagé + moderation-service lui-même.
# Les clients user-service et auth-service utilisent des stubs proto
# auto-contenus dans services/moderation-service/proto/gen/.
COPY pkg/go.mod pkg/go.sum ./pkg/
COPY services/moderation-service/go.mod services/moderation-service/go.sum* ./services/moderation-service/

RUN printf 'go 1.25.6\n\nuse (\n\t./pkg\n\t./services/moderation-service\n)\n' > go.work

RUN go mod download -C ./pkg
RUN go mod download -C ./services/moderation-service

# =============================================================================
# Stage 2: Builder
# =============================================================================
FROM deps AS builder

ARG VERSION=dev
ARG COMMIT_SHA=unknown
ARG BUILD_TIME

WORKDIR /build

COPY pkg/ ./pkg/
COPY services/moderation-service/ ./services/moderation-service/

WORKDIR /build/services/moderation-service

RUN BUILD_TIME=${BUILD_TIME:-$(date -u +%Y-%m-%dT%H:%M:%SZ)} && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s \
        -X main.version=${VERSION} \
        -X main.commitSHA=${COMMIT_SHA} \
        -X main.buildTime=${BUILD_TIME}" \
    -trimpath \
    -o /build/moderation-service \
    ./cmd/server/main.go

# =============================================================================
# Stage 3: Runtime
# =============================================================================
FROM alpine:${ALPINE_VERSION} AS runtime

ARG VERSION=dev
ARG COMMIT_SHA=unknown
ARG BUILD_TIME

LABEL org.opencontainers.image.title="moderation-service" \
      org.opencontainers.image.description="tissiMah Moderation Service — filtre textes et images" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT_SHA}" \
      org.opencontainers.image.created="${BUILD_TIME}" \
      org.opencontainers.image.source="https://github.com/Kpeewu/tissi-mah" \
      org.opencontainers.image.vendor="tissiMah" \
      org.opencontainers.image.licenses="MIT"

RUN apk --no-cache add ca-certificates tzdata && \
    update-ca-certificates && rm -rf /var/cache/apk/*

RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser -h /app -s /sbin/nologin appuser

WORKDIR /app

COPY --from=builder --chown=appuser:appuser /build/moderation-service .
COPY --from=builder --chown=appuser:appuser /build/services/moderation-service/migrations ./migrations/
COPY --from=migrate/migrate:v4.17.0 /usr/local/bin/migrate /usr/local/bin/migrate
COPY --from=ghcr.io/grpc-ecosystem/grpc-health-probe:v0.4.24 /ko-app/grpc-health-probe /usr/local/bin/grpc_health_probe

USER appuser

EXPOSE 50066

ENV ENVIRONMENT=production \
    LOG_LEVEL=info \
    GRPC_PORT=50066

ENTRYPOINT ["/app/moderation-service"]
