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

require (
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/pelletier/go-toml/v2 v2.2.4 // indirect
	github.com/sagikazarmark/locafero v0.12.0 // indirect
	github.com/spf13/afero v1.15.0 // indirect
	github.com/spf13/cast v1.10.0 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/spf13/viper v1.21.0 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/sys v0.40.0 // indirect
)

require (
	github.com/Kpeewu/tissi-mah/pkg v0.0.0-00010101000000-000000000000
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	go.uber.org/zap v1.27.1
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/text v0.33.0 // indirect
)
