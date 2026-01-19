# Multi-stage build pour optimiser la taille

# Stage 1: Build
FROM golang:1.21-alpine AS builder

# Metadata
LABEL stage=builder

# Install dependencies
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build

# Copy go mod files first (for caching)
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build with optimizations
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.version=${VERSION:-dev} -X main.buildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -o auth-service \
    ./cmd/server/main.go

# Stage 2: Runtime
FROM alpine:3.19

# Install CA certificates for HTTPS/TLS and timezone data
RUN apk --no-cache add ca-certificates tzdata && \
    update-ca-certificates

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/auth-service .

# Copy migrations
COPY --from=builder /build/migrations ./migrations

# Security: Create non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser && \
    chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Expose gRPC port
EXPOSE 50051

# Healthcheck
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD ["/app/auth-service", "health"] || exit 1

# Run
ENTRYPOINT ["/app/auth-service"]