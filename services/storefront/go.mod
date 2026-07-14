module github.com/channel-manager/channel-manager/services/storefront

go 1.25.0

replace (
	github.com/channel-manager/channel-manager/platform/auth => ../../platform/auth
	github.com/channel-manager/channel-manager/platform/db => ../../platform/db
	github.com/channel-manager/channel-manager/platform/events => ../../platform/events
	github.com/channel-manager/channel-manager/services/audit => ../audit
	github.com/channel-manager/channel-manager/services/inventory => ../inventory
	github.com/channel-manager/channel-manager/services/pms => ../pms
	github.com/channel-manager/channel-manager/services/pricing => ../pricing
	github.com/channel-manager/channel-manager/services/reservations => ../reservations
)

require (
	github.com/channel-manager/channel-manager/platform/auth v0.0.0-00010101000000-000000000000
	github.com/channel-manager/channel-manager/services/audit v0.0.0-00010101000000-000000000000
	github.com/channel-manager/channel-manager/services/pms v0.0.0-00010101000000-000000000000
	github.com/channel-manager/channel-manager/services/pricing v0.0.0-00010101000000-000000000000
	github.com/channel-manager/channel-manager/services/reservations v0.0.0-00010101000000-000000000000
	github.com/google/uuid v1.6.0
	github.com/redis/go-redis/v9 v9.7.3
)

require (
	connectrpc.com/connect v1.19.2 // indirect
	github.com/MicahParks/jwkset v0.11.0 // indirect
	github.com/MicahParks/keyfunc/v3 v3.8.0 // indirect
	github.com/bmatcuk/doublestar/v4 v4.6.1 // indirect
	github.com/casbin/casbin/v3 v3.10.0 // indirect
	github.com/casbin/govaluate v1.3.0 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/golang-jwt/jwt/v5 v5.3.1 // indirect
	github.com/google/go-querystring v1.2.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/pgx/v5 v5.9.2 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/workos/workos-go/v7 v7.1.0 // indirect
	golang.org/x/sync v0.17.0 // indirect
	golang.org/x/text v0.29.0 // indirect
	golang.org/x/time v0.9.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)
