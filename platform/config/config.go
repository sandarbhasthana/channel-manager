package config

import (
	"os"
	"strconv"
)

// AppConfig is the top-level configuration for the application.
type AppConfig struct {
	DB            DBConfig
	Redis         RedisConfig
	NATS          NATSConfig
	Auth          AuthConfig
	Integration   IntegrationConfig
	Observability ObservabilityConfig
}

// IntegrationConfig holds PMS ↔ Channel Manager machine auth settings.
type IntegrationConfig struct {
	// SecretsJSON is the raw CM_INTEGRATION_SECRETS env value (org_id → token map).
	SecretsJSON string
}

// DBConfig holds PostgreSQL connection settings.
//
// Two credential pairs are tracked: User/Password is the privileged
// identity used by the migration runner (typically `postgres`), while
// RuntimeUser/RuntimePassword is the unprivileged identity bound by
// long-running services (typically `app`). The split lets RLS policies
// be enforced on application traffic without giving the runtime the
// ability to alter schema or bypass policies.
type DBConfig struct {
	Host            string
	Port            int
	DBName          string
	User            string
	Password        string
	SSLMode         string
	RuntimeUser     string
	RuntimePassword string
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

// NATSConfig holds NATS connection settings.
type NATSConfig struct {
	URL string
}

// AuthConfig holds authentication settings. Identity is provided by an
// external IdP (WorkOS); the backend only verifies JWTs against the IdP's
// JWKS and never holds a signing secret.
type AuthConfig struct {
	// Provider is a free-form tag (e.g. "workos") used by tooling. Optional.
	Provider string
	// Issuer is the expected `iss` claim. For WorkOS this is
	// "https://api.workos.com".
	Issuer string
	// JWKSURL is the URL of the IdP's JSON Web Key Set. For WorkOS this is
	// derived from the client id: https://api.workos.com/sso/jwks/<client_id>.
	JWKSURL string

	// WorkOS-specific server-side credentials. Required to perform the
	// OAuth code exchange in /auth/callback and to call the WorkOS
	// management APIs (e.g. organization listing). Not used for token
	// verification.
	WorkOSAPIKey         string
	WorkOSClientID       string
	WorkOSCookiePassword string
	WorkOSRedirectURI    string
	// WorkOSWebhookSecret is the signing secret WorkOS uses to sign
	// webhook payloads. Required to verify incoming webhooks; the
	// handler rejects requests whose HMAC does not match.
	WorkOSWebhookSecret string
}

// ObservabilityConfig holds observability settings.
type ObservabilityConfig struct {
	ServiceName  string
	OTLPEndpoint string
	SentryDSN    string
}

// Load reads configuration from environment variables and returns an AppConfig.
func Load() (*AppConfig, error) {
	dbPort, _ := strconv.Atoi(getEnv("DB_PORT", "5432"))
	redisDB, _ := strconv.Atoi(getEnv("REDIS_DB", "0"))

	cfg := &AppConfig{
		DB: DBConfig{
			Host:            getEnv("DB_HOST", "localhost"),
			Port:            dbPort,
			DBName:          getEnv("DB_NAME", "channel_manager"),
			User:            getEnv("DB_USER", "postgres"),
			Password:        getEnv("DB_PASSWORD", ""),
			SSLMode:         getEnv("DB_SSLMODE", "disable"),
			RuntimeUser:     getEnv("APP_DB_USER", "app"),
			RuntimePassword: getEnv("APP_DB_PASSWORD", "app_dev"),
		},
		Redis: RedisConfig{
			Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       redisDB,
		},
		NATS: NATSConfig{
			URL: getEnv("NATS_URL", "nats://localhost:4222"),
		},
		Auth: AuthConfig{
			Provider:             getEnv("AUTH_PROVIDER", "workos"),
			Issuer:               getEnv("AUTH_ISSUER", "https://api.workos.com"),
			JWKSURL:              getEnv("AUTH_JWKS_URL", ""),
			WorkOSAPIKey:         getEnv("WORKOS_API_KEY", ""),
			WorkOSClientID:       getEnv("WORKOS_CLIENT_ID", ""),
			WorkOSCookiePassword: getEnv("WORKOS_COOKIE_PASSWORD", ""),
			WorkOSRedirectURI:    getEnv("WORKOS_REDIRECT_URI", "http://localhost:8080/auth/callback"),
			WorkOSWebhookSecret:  getEnv("WORKOS_WEBHOOK_SECRET", ""),
		},
		Integration: IntegrationConfig{
			SecretsJSON: getEnv("CM_INTEGRATION_SECRETS", ""),
		},
		Observability: ObservabilityConfig{
			ServiceName:  getEnv("OTEL_SERVICE_NAME", "channel-manager"),
			OTLPEndpoint: getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317"),
			SentryDSN:    getEnv("SENTRY_DSN", ""),
		},
	}

	return cfg, nil
}

// getEnv returns the value of an environment variable or a default value.
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
