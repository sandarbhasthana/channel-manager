module github.com/channel-manager/channel-manager/services/inventory

go 1.24.0

require (
	connectrpc.com/connect v1.19.2
	github.com/channel-manager/channel-manager/gen/go v0.0.0-00010101000000-000000000000
	github.com/channel-manager/channel-manager/platform/auth v0.0.0-00010101000000-000000000000
	github.com/channel-manager/channel-manager/platform/db v0.0.0-00010101000000-000000000000
	github.com/channel-manager/channel-manager/platform/events v0.0.0-00010101000000-000000000000
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.7.5
	github.com/redis/go-redis/v9 v9.7.3
	google.golang.org/protobuf v1.36.11
)

require (
	github.com/MicahParks/jwkset v0.11.0 // indirect
	github.com/MicahParks/keyfunc/v3 v3.8.0 // indirect
	github.com/bmatcuk/doublestar/v4 v4.6.1 // indirect
	github.com/casbin/casbin/v3 v3.10.0 // indirect
	github.com/casbin/govaluate v1.3.0 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/golang-jwt/jwt/v5 v5.3.1 // indirect
	github.com/golang-migrate/migrate/v4 v4.18.3 // indirect
	github.com/google/go-querystring v1.2.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/lib/pq v1.10.9 // indirect
	github.com/workos/workos-go/v7 v7.1.0 // indirect
	go.uber.org/atomic v1.7.0 // indirect
	golang.org/x/crypto v0.37.0 // indirect
	golang.org/x/sync v0.13.0 // indirect
	golang.org/x/text v0.24.0 // indirect
	golang.org/x/time v0.9.0 // indirect
)

replace (
	github.com/channel-manager/channel-manager/gen/go => ../../gen/go
	github.com/channel-manager/channel-manager/platform/auth => ../../platform/auth
	github.com/channel-manager/channel-manager/platform/config => ../../platform/config
	github.com/channel-manager/channel-manager/platform/db => ../../platform/db
	github.com/channel-manager/channel-manager/platform/events => ../../platform/events
)
