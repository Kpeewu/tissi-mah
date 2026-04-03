ARG GO_VERSION=1.25
ARG ALPINE_VERSION=3.21

# Stage 1: Dependencies
FROM golang:${GO_VERSION}-alpine AS deps
WORKDIR /build
RUN apk add --no-cache git ca-certificates tzdata
COPY pkg/go.mod pkg/go.sum ./pkg/
COPY services/notification-service/go.mod services/notification-service/go.sum* ./services/notification-service/
COPY services/user-service/go.mod services/user-service/go.sum* ./services/user-service/
COPY services/push-service/go.mod services/push-service/go.sum* ./services/push-service/
COPY services/email-service/go.mod services/email-service/go.sum* ./services/email-service/
RUN printf 'go 1.25.6\n\nuse (\n\t./pkg\n\t./services/notification-service\n\t./services/user-service\n\t./services/push-service\n\t./services/email-service\n)\n' > go.work
RUN go mod download -C ./pkg
RUN go mod download -C ./services/notification-service

# Stage 2: Builder
FROM deps AS builder
ARG VERSION=dev
ARG COMMIT_SHA=unknown
ARG BUILD_TIME
WORKDIR /build
COPY pkg/ ./pkg/
COPY services/notification-service/ ./services/notification-service/
COPY services/user-service/proto/ ./services/user-service/proto/
COPY services/push-service/proto/ ./services/push-service/proto/
COPY services/email-service/proto/ ./services/email-service/proto/
WORKDIR /build/services/notification-service
RUN BUILD_TIME=${BUILD_TIME:-$(date -u +%Y-%m-%dT%H:%M:%SZ)} && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s \
        -X main.version=${VERSION} \
        -X main.commitSHA=${COMMIT_SHA} \
        -X main.buildTime=${BUILD_TIME}" \
    -trimpath \
    -o /build/notification-service \
    ./cmd/server/main.go

# Stage 3: Runtime
FROM alpine:${ALPINE_VERSION} AS runtime
ARG VERSION=dev
ARG COMMIT_SHA=unknown
ARG BUILD_TIME
LABEL org.opencontainers.image.title="notification-service"
RUN apk --no-cache add ca-certificates tzdata && update-ca-certificates
RUN addgroup -g 1000 appuser && adduser -D -u 1000 -G appuser -h /app -s /sbin/nologin appuser
WORKDIR /app
COPY --from=builder --chown=appuser:appuser /build/notification-service .
USER appuser
EXPOSE 50060
ENV ENVIRONMENT=production LOG_LEVEL=info GRPC_PORT=50060
ENTRYPOINT ["/app/notification-service"]
CMD ["serve"]
