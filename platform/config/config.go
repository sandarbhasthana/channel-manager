package config

import (
	"os"
	"strconv"
	"time"
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
	// Development only, and ignored unless CM_ALLOW_DEV_INTEGRATION_SECRETS=true.
	SecretsJSON string
	// PmsPublicKeysJSON maps key id → Ed25519 public key (PEM or base64), used to
	// verify assertions from a bundled tenant's PMS. A map so a new key can be
	// published here before the PMS starts signing with it, which is what makes
	// rotation possible without restarting both services at the same instant.
	PmsPublicKeysJSON string
	// PmsIssuer and PmsAudience are the expected `iss` and `aud`. Empty disables
	// the respective check.
	PmsIssuer   string
	PmsAudience string
	// PmsClockSkew tolerates drift between the PMS and this service. Assertions
	// live ~90s, so with no leeway a few seconds of drift between two containers
	// rejects valid tokens intermittently — which looks like flaky 401s under
	// load rather than like a misconfiguration.
	PmsClockSkew time.Duration
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
	// Issuer is the exact expected `iss` claim. WorkOS User Management tokens
	// use https://api.workos.com/user_management/<client_id>.
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
	// WorkOSOrganizationID pins browser sessions to the tenant used by this
	// deployment. Without it, social OAuth can return a user-level token with no
	// org_id, which is unusable by the tenant-scoped API.
	WorkOSOrganizationID string
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
	workOSClientID := getEnv("WORKOS_CLIENT_ID", "")
	workOSIssuer := ""
	if workOSClientID != "" {
		workOSIssuer = "https://api.workos.com/user_management/" + workOSClientID
	}

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
			Issuer:               getEnv("AUTH_ISSUER", workOSIssuer),
			JWKSURL:              getEnv("AUTH_JWKS_URL", ""),
			WorkOSAPIKey:         getEnv("WORKOS_API_KEY", ""),
			WorkOSClientID:       workOSClientID,
			WorkOSCookiePassword: getEnv("WORKOS_COOKIE_PASSWORD", ""),
			WorkOSRedirectURI:    getEnv("WORKOS_REDIRECT_URI", "http://localhost:8080/auth/callback"),
			WorkOSOrganizationID: getEnv("WORKOS_ORG_ID", ""),
			WorkOSWebhookSecret:  getEnv("WORKOS_WEBHOOK_SECRET", ""),
		},
		Integration: IntegrationConfig{
			SecretsJSON:       getEnv("CM_INTEGRATION_SECRETS", ""),
			PmsPublicKeysJSON: getEnv("CM_PMS_PUBLIC_KEYS", ""),
			PmsIssuer:         getEnv("CM_PMS_JWT_ISSUER", "pms"),
			PmsAudience:       getEnv("CM_PMS_JWT_AUDIENCE", "channel-manager"),
			PmsClockSkew:      getEnvDuration("CM_PMS_JWT_CLOCK_SKEW", 30*time.Second),
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

// getEnvDuration parses a Go duration string ("30s", "2m"). An unparseable value
// falls back to the default rather than failing startup: this is a tolerance
// knob, and refusing to boot over a typo in it would trade a small
// misconfiguration for a total outage.
func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return defaultValue
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil || parsed < 0 {
		return defaultValue
	}
	return parsed
}
