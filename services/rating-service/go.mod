module github.com/Kpeewu/tissi-mah/services/rating-service

go 1.25.5

// =============================================================================
// SERVICE-SPECIFIC DEPENDENCIES
// Add dependencies here that are specific to rating-service only.
// Common dependencies should be in the shared pkg module.
// =============================================================================

// =============================================================================
// WORKSPACE REPLACE DIRECTIVE
// This allows the service to use the local pkg module during development.
// In production builds, this is handled by the go.work file.
// =============================================================================

replace github.com/Kpeewu/tissi-mah/pkg => ../../pkg

require github.com/jackc/pgx/v5 v5.8.0

require go.uber.org/multierr v1.10.0 // indirect

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	go.uber.org/zap v1.27.1
	golang.org/x/sync v0.17.0 // indirect
	golang.org/x/text v0.29.0 // indirect
)
