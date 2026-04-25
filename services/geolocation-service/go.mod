module github.com/Kpeewu/tissi-mah/services/geolocation-service

go 1.25.5

// =============================================================================
// SERVICE-SPECIFIC DEPENDENCIES
// =============================================================================

// =============================================================================
// WORKSPACE REPLACE DIRECTIVE
// =============================================================================

replace github.com/Kpeewu/tissi-mah/pkg => ../../pkg

replace github.com/Kpeewu/tissi-mah/pkg-test => ../../pkg-test

require (
	github.com/Kpeewu/tissi-mah/pkg v0.0.0-00010101000000-000000000000
	github.com/sony/gobreaker v1.0.0
	go.uber.org/zap v1.27.1
	golang.org/x/sync v0.20.0
	google.golang.org/genproto/googleapis/api v0.0.0-20260316180232-0b37fe3546d5
	google.golang.org/grpc v1.79.3
	google.golang.org/protobuf v1.36.11
)

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
	go.opentelemetry.io/otel/metric v1.42.0 // indirect
	go.opentelemetry.io/otel/sdk v1.42.0 // indirect
	go.opentelemetry.io/otel/trace v1.42.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/net v0.52.0 // indirect
	golang.org/x/sys v0.42.0 // indirect
	golang.org/x/text v0.35.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260319201613-d00831a3d3e7 // indirect
)
