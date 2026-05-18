module github.com/channel-manager/channel-manager/apps/api

go 1.25.0

require (
	connectrpc.com/connect v1.19.2
	github.com/channel-manager/channel-manager/gen/go v0.0.0-00010101000000-000000000000
	github.com/channel-manager/channel-manager/platform/auth v0.0.0-00010101000000-000000000000
	github.com/channel-manager/channel-manager/platform/config v0.0.0-00010101000000-000000000000
	github.com/channel-manager/channel-manager/platform/db v0.0.0-00010101000000-000000000000
	github.com/channel-manager/channel-manager/platform/events v0.0.0-00010101000000-000000000000
	github.com/channel-manager/channel-manager/platform/integration v0.0.0-00010101000000-000000000000
	github.com/channel-manager/channel-manager/platform/observability v0.0.0-00010101000000-000000000000
	github.com/channel-manager/channel-manager/services/channel v0.0.0-00010101000000-000000000000
	github.com/channel-manager/channel-manager/services/integration v0.0.0-00010101000000-000000000000
	github.com/channel-manager/channel-manager/services/inventory v0.0.0-00010101000000-000000000000
	github.com/channel-manager/channel-manager/services/pms v0.0.0-00010101000000-000000000000
	github.com/channel-manager/channel-manager/services/reservations v0.0.0-00010101000000-000000000000
	github.com/redis/go-redis/v9 v9.7.3
	github.com/workos/workos-go/v7 v7.1.0
)

require (
	github.com/MicahParks/jwkset v0.11.0 // indirect
	github.com/MicahParks/keyfunc/v3 v3.8.0 // indirect
	github.com/bmatcuk/doublestar/v4 v4.6.1 // indirect
	github.com/casbin/casbin/v3 v3.10.0 // indirect
	github.com/casbin/govaluate v1.3.0 // indirect
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/getsentry/sentry-go v0.46.2 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/golang-jwt/jwt/v5 v5.3.1 // indirect
	github.com/golang-migrate/migrate/v4 v4.18.3 // indirect
	github.com/google/go-querystring v1.2.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.28.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/pgx/v5 v5.9.2 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/lib/pq v1.10.9 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/otel v1.43.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc v1.43.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.43.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.43.0 // indirect
	go.opentelemetry.io/otel/metric v1.43.0 // indirect
	go.opentelemetry.io/otel/sdk v1.43.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.43.0 // indirect
	go.opentelemetry.io/otel/trace v1.43.0 // indirect
	go.opentelemetry.io/proto/otlp v1.10.0 // indirect
	go.uber.org/atomic v1.7.0 // indirect
	golang.org/x/crypto v0.49.0 // indirect
	golang.org/x/net v0.52.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/sys v0.42.0 // indirect
	golang.org/x/text v0.35.0 // indirect
	golang.org/x/time v0.9.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260401024825-9d38bb4040a9 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260401024825-9d38bb4040a9 // indirect
	google.golang.org/grpc v1.80.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)

replace (
	github.com/channel-manager/channel-manager/gen/go => ../../gen/go
	github.com/channel-manager/channel-manager/platform/auth => ../../platform/auth
	github.com/channel-manager/channel-manager/platform/config => ../../platform/config
	github.com/channel-manager/channel-manager/platform/db => ../../platform/db
	github.com/channel-manager/channel-manager/platform/events => ../../platform/events
	github.com/channel-manager/channel-manager/platform/integration => ../../platform/integration
	github.com/channel-manager/channel-manager/platform/observability => ../../platform/observability
	github.com/channel-manager/channel-manager/services/channel => ../../services/channel
	github.com/channel-manager/channel-manager/services/integration => ../../services/integration
	github.com/channel-manager/channel-manager/services/inventory => ../../services/inventory
	github.com/channel-manager/channel-manager/services/pms => ../../services/pms
	github.com/channel-manager/channel-manager/services/reservations => ../../services/reservations
)
