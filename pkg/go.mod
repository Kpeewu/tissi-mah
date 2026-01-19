module github.com/Kpeewu/tissi-mah/pkg

go 1.21

// =============================================================================
// SHARED DEPENDENCIES
// These dependencies are common to all services in the monorepo.
// Add dependencies here that are used by multiple services.
// =============================================================================

require (
	// Database
	github.com/jackc/pgx/v5 v5.5.1
	github.com/redis/go-redis/v9 v9.3.1

	// gRPC
	google.golang.org/grpc v1.60.1
	google.golang.org/protobuf v1.32.0

	// Logging
	go.uber.org/zap v1.26.0

	// Configuration
	github.com/spf13/viper v1.18.2

	// Validation
	github.com/go-playground/validator/v10 v10.16.0

	// Observability
	go.opentelemetry.io/otel v1.21.0
	go.opentelemetry.io/otel/trace v1.21.0
	github.com/prometheus/client_golang v1.18.0

	// Utilities
	github.com/google/uuid v1.5.0
)

// Indirect dependencies will be added by go mod tidy
