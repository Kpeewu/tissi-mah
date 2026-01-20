module github.com/Kpeewu/tissi-mah/services/auth-service

go 1.21

// =============================================================================
// SERVICE-SPECIFIC DEPENDENCIES
// Add dependencies here that are specific to auth-service only.
// Common dependencies should be in the shared pkg module.
// =============================================================================

require (
	// Shared packages (common dependencies)
	github.com/Kpeewu/tissi-mah/pkg v0.0.0

	// Auth-specific dependencies
	firebase.google.com/go/v4 v4.13.0 // Firebase Admin SDK for phone auth
	github.com/golang-jwt/jwt/v5 v5.2.0 // JWT handling
	golang.org/x/crypto v0.17.0 // Password hashing (bcrypt, argon2)
)

// =============================================================================
// WORKSPACE REPLACE DIRECTIVE
// This allows the service to use the local pkg module during development.
// In production builds, this is handled by the go.work file.
// =============================================================================

replace github.com/Kpeewu/tissi-mah/pkg => ../../pkg
