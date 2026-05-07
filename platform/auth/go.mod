module github.com/channel-manager/channel-manager/platform/auth

go 1.24.0

require (
	connectrpc.com/connect v1.19.2
	github.com/MicahParks/keyfunc/v3 v3.8.0
	github.com/casbin/casbin/v3 v3.10.0
	github.com/golang-jwt/jwt/v5 v5.3.1
	github.com/jackc/pgx/v5 v5.7.5
	github.com/workos/workos-go/v7 v7.1.0
)

require (
	github.com/MicahParks/jwkset v0.11.0 // indirect
	github.com/bmatcuk/doublestar/v4 v4.6.1 // indirect
	github.com/casbin/govaluate v1.3.0 // indirect
	github.com/google/go-querystring v1.2.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	golang.org/x/crypto v0.37.0 // indirect
	golang.org/x/sync v0.13.0 // indirect
	golang.org/x/text v0.24.0 // indirect
	golang.org/x/time v0.9.0 // indirect
	google.golang.org/protobuf v1.36.9 // indirect
)

replace github.com/channel-manager/channel-manager/platform/config => ../config

replace github.com/channel-manager/channel-manager/platform/db => ../db
