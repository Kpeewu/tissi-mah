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
