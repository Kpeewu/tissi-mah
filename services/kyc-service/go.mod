module github.com/Kpeewu/tissi-mah/services/kyc-service

go 1.25.5

// =============================================================================
// SERVICE-SPECIFIC DEPENDENCIES
// Add dependencies here that are specific to kyc-service only.
// Common dependencies should be in the shared pkg module.
// =============================================================================

require (
	github.com/stretchr/testify v1.11.1
	go.uber.org/zap v1.27.1
)

require (
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

// =============================================================================
// WORKSPACE REPLACE DIRECTIVE
// This allows the service to use the local pkg module during development.
// In production builds, this is handled by the go.work file.
// =============================================================================

replace github.com/Kpeewu/tissi-mah/pkg => ../../pkg
