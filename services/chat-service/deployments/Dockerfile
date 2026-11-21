ARG GO_VERSION=1.25
ARG ALPINE_VERSION=3.21

# =============================================================================
# Stage 1: Dependencies
# =============================================================================
FROM golang:${GO_VERSION}-alpine AS deps

WORKDIR /build

RUN apk add --no-cache git ca-certificates tzdata

# go.mod des modules requis (toutes les replace directives transitives) :
# - chat-service replace booking-service, trips-service, support-service
# - booking-service replace user-service, payment-service
# - trips-service replace user-service, vehicle-service, rating-service
# - vehicle-service replace file-service
# - file-service replace user-service
# - support-service replace email-service
# - payment-service replace user-service, support-service
COPY pkg/go.mod pkg/go.sum ./pkg/
COPY services/file-service/go.mod services/file-service/go.sum* ./services/file-service/
COPY services/user-service/go.mod services/user-service/go.sum* ./services/user-service/
COPY services/vehicle-service/go.mod services/vehicle-service/go.sum* ./services/vehicle-service/
COPY services/trips-service/go.mod services/trips-service/go.sum* ./services/trips-service/
COPY services/rating-service/go.mod services/rating-service/go.sum* ./services/rating-service/
COPY services/booking-service/go.mod services/booking-service/go.sum* ./services/booking-service/
COPY services/email-service/go.mod services/email-service/go.sum* ./services/email-service/
COPY services/support-service/go.mod services/support-service/go.sum* ./services/support-service/
COPY services/payment-service/go.mod services/payment-service/go.sum* ./services/payment-service/
COPY services/chat-service/go.mod services/chat-service/go.sum* ./services/chat-service/

RUN mkdir -p pkg-test && \
    printf 'module github.com/Kpeewu/tissi-mah/pkg-test\n\ngo 1.25\n' > pkg-test/go.mod

RUN printf 'go 1.25.6\n\nuse (\n\t./pkg\n\t./pkg-test\n\t./services/file-service\n\t./services/user-service\n\t./services/vehicle-service\n\t./services/trips-service\n\t./services/rating-service\n\t./services/booking-service\n\t./services/email-service\n\t./services/support-service\n\t./services/payment-service\n\t./services/chat-service\n)\n' > go.work

RUN go mod download -C ./pkg
RUN go mod download -C ./services/file-service
RUN go mod download -C ./services/user-service
RUN go mod download -C ./services/vehicle-service
RUN go mod download -C ./services/trips-service
RUN go mod download -C ./services/rating-service
RUN go mod download -C ./services/booking-service
RUN go mod download -C ./services/email-service
RUN go mod download -C ./services/support-service
RUN go mod download -C ./services/payment-service
RUN go mod download -C ./services/chat-service

# =============================================================================
# Stage 2: Builder
# =============================================================================
FROM deps AS builder

ARG VERSION=dev
ARG COMMIT_SHA=unknown
ARG BUILD_TIME

WORKDIR /build

COPY pkg/ ./pkg/
COPY services/file-service/ ./services/file-service/
COPY services/user-service/ ./services/user-service/
COPY services/vehicle-service/ ./services/vehicle-service/
COPY services/trips-service/ ./services/trips-service/
COPY services/rating-service/ ./services/rating-service/
COPY services/booking-service/ ./services/booking-service/
COPY services/email-service/ ./services/email-service/
COPY services/support-service/ ./services/support-service/
COPY services/payment-service/ ./services/payment-service/
COPY services/chat-service/ ./services/chat-service/

WORKDIR /build/services/chat-service

RUN BUILD_TIME=${BUILD_TIME:-$(date -u +%Y-%m-%dT%H:%M:%SZ)} && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -trimpath \
    -o /build/chat-service \
    ./cmd/server/main.go

# =============================================================================
# Stage 3: Runtime
# =============================================================================
FROM alpine:${ALPINE_VERSION} AS runtime

ARG VERSION=dev

LABEL org.opencontainers.image.title="chat-service" \
      org.opencontainers.image.description="tissiMah Chat Service" \
      org.opencontainers.image.version="${VERSION}"

RUN apk --no-cache add ca-certificates tzdata && \
    update-ca-certificates && rm -rf /var/cache/apk/*

RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser -h /app -s /sbin/nologin appuser

WORKDIR /app

COPY --from=builder --chown=appuser:appuser /build/chat-service .
COPY --from=builder --chown=appuser:appuser /build/services/chat-service/migrations ./migrations/
COPY --from=migrate/migrate:v4.17.0 /usr/local/bin/migrate /usr/local/bin/migrate
COPY --from=ghcr.io/grpc-ecosystem/grpc-health-probe:v0.4.24 /ko-app/grpc-health-probe /usr/local/bin/grpc_health_probe

USER appuser
EXPOSE 50065

ENV ENVIRONMENT=production \
    LOG_LEVEL=info \
    GRPC_PORT=50065

ENTRYPOINT ["/app/chat-service"]
