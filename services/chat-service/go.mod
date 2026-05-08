module github.com/Kpeewu/tissi-mah/services/chat-service

go 1.25.6

replace github.com/Kpeewu/tissi-mah/pkg => ../../pkg
replace github.com/Kpeewu/tissi-mah/services/booking-service => ../booking-service
replace github.com/Kpeewu/tissi-mah/services/trips-service => ../trips-service
replace github.com/Kpeewu/tissi-mah/services/support-service => ../support-service

require (
	github.com/Kpeewu/tissi-mah/pkg v0.0.0-00010101000000-000000000000
	github.com/Kpeewu/tissi-mah/services/booking-service v0.0.0-00010101000000-000000000000
	github.com/Kpeewu/tissi-mah/services/trips-service v0.0.0-00010101000000-000000000000
	github.com/Kpeewu/tissi-mah/services/support-service v0.0.0-00010101000000-000000000000
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.9.1
	github.com/redis/go-redis/v9 v9.18.0
	github.com/spf13/viper v1.21.0
	github.com/stretchr/testify v1.11.1
	go.uber.org/zap v1.27.1
	golang.org/x/crypto v0.49.0
	google.golang.org/genproto/googleapis/api v0.0.0-20260316180232-0b37fe3546d5
	google.golang.org/grpc v1.79.3
	google.golang.org/protobuf v1.36.11
)
