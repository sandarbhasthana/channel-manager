package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"github.com/redis/go-redis/v9"
	workos "github.com/workos/workos-go/v7"

	"github.com/channel-manager/channel-manager/gen/go/channel/v1/channelv1connect"
	"github.com/channel-manager/channel-manager/gen/go/inventory/v1/inventoryv1connect"
	"github.com/channel-manager/channel-manager/gen/go/pms/v1/pmsv1connect"
	"github.com/channel-manager/channel-manager/gen/go/pricing/v1/pricingv1connect"
	"github.com/channel-manager/channel-manager/platform/auth"
	"github.com/channel-manager/channel-manager/platform/config"
	"github.com/channel-manager/channel-manager/platform/db"
	platformevents "github.com/channel-manager/channel-manager/platform/events"
	"github.com/channel-manager/channel-manager/platform/events/inproc"
	platformintegration "github.com/channel-manager/channel-manager/platform/integration"
	"github.com/channel-manager/channel-manager/platform/observability"
	"github.com/channel-manager/channel-manager/services/channel/adapters/agoda"
	"github.com/channel-manager/channel-manager/services/channel/adapters/airbnb"
	"github.com/channel-manager/channel-manager/services/channel/adapters/bookingcom"
	channelconnect "github.com/channel-manager/channel-manager/services/channel/adapters/connect"
	channelevents "github.com/channel-manager/channel-manager/services/channel/adapters/events"
	"github.com/channel-manager/channel-manager/services/channel/adapters/expedia"
	"github.com/channel-manager/channel-manager/services/channel/adapters/infra"
	channelpostgres "github.com/channel-manager/channel-manager/services/channel/adapters/postgres"
	channelusecases "github.com/channel-manager/channel-manager/services/channel/usecases"
	inventoryconnect "github.com/channel-manager/channel-manager/services/inventory/adapters/connect"
	inventoryevents "github.com/channel-manager/channel-manager/services/inventory/adapters/events"
	inventorypostgres "github.com/channel-manager/channel-manager/services/inventory/adapters/postgres"
	inventoryredis "github.com/channel-manager/channel-manager/services/inventory/adapters/redis"
	"github.com/channel-manager/channel-manager/services/inventory/usecases"
	pmsconnect "github.com/channel-manager/channel-manager/services/pms/adapters/connect"
	pmsinventory "github.com/channel-manager/channel-manager/services/pms/adapters/inventory"
	pmsinfra "github.com/channel-manager/channel-manager/services/pms/adapters/infra"
	pmspostgres "github.com/channel-manager/channel-manager/services/pms/adapters/postgres"
	pmsusecases "github.com/channel-manager/channel-manager/services/pms/usecases"
	integrationhttp "github.com/channel-manager/channel-manager/services/integration/adapters/http"
	integrationusecases "github.com/channel-manager/channel-manager/services/integration/usecases"
	resevents "github.com/channel-manager/channel-manager/services/reservations/adapters/events"
	respostgres "github.com/channel-manager/channel-manager/services/reservations/adapters/postgres"
	resusecases "github.com/channel-manager/channel-manager/services/reservations/usecases"
	pricingconnect "github.com/channel-manager/channel-manager/services/pricing/adapters/connect"
)

func main() {
	slog.Info("Channel Manager API starting...")

	cfg, err := config.Load()
	must(err, "load config")

	// ── Observability (OTel traces + metrics + Sentry) ────────────────────────
	// Initialised before anything else so that spans are captured from startup.
	must(observability.Init(observability.Config{
		ServiceName:  cfg.Observability.ServiceName,
		OTLPEndpoint: cfg.Observability.OTLPEndpoint,
		SentryDSN:    cfg.Observability.SentryDSN,
	}), "init observability")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ── Database pool (runtime app role — RLS enforced) ──────────────────────
	pool, err := db.NewPool(ctx, db.Config{
		Host:     cfg.DB.Host,
		Port:     cfg.DB.Port,
		DBName:   cfg.DB.DBName,
		User:     cfg.DB.RuntimeUser,
		Password: cfg.DB.RuntimePassword,
		SSLMode:  cfg.DB.SSLMode,
	})
	must(err, "open runtime db pool")
	defer pool.Close()

	// ── WorkOS client ─────────────────────────────────────────────────────────
	wos := workos.NewClient(cfg.Auth.WorkOSAPIKey, workos.WithClientID(cfg.Auth.WorkOSClientID))

	// ── JWT verifier (JWKS + RS256) ───────────────────────────────────────────
	// WorkOS AuthKit access tokens do not include an `aud` claim, so the
	// audience check is left empty (disabled). The issuer is validated against
	// AUTH_ISSUER which must match the token's `iss`.
	verifier, err := auth.NewVerifier(ctx, auth.VerifierConfig{
		JWKSURL: cfg.Auth.JWKSURL,
		Issuer:  cfg.Auth.Issuer,
	})
	must(err, "init jwt verifier")

	// ── Identity store (WorkOS ↔ tenancy mirror) ──────────────────────────────
	store := auth.NewStore(pool.Inner())

	// ── Casbin enforcer ───────────────────────────────────────────────────────
	enforcer, err := auth.NewEnforcer(pool.Inner())
	must(err, "init casbin enforcer")

	// ── HTTP mux ──────────────────────────────────────────────────────────────
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"ok"}`)
	})

	// Public auth routes.
	mux.Handle("GET /auth/login", auth.LoginHandler(wos, cfg.Auth.WorkOSClientID, cfg.Auth.WorkOSRedirectURI))
	mux.Handle("GET /auth/callback", auth.CallbackHandler(wos, store))
	mux.Handle("POST /auth/webhook", auth.NewWebhookHandler(cfg.Auth.WorkOSWebhookSecret, store))

	// Password login — cross-origin fetch from the dashboard (CORS required).
	dashboardOrigin := os.Getenv("DASHBOARD_URL")
	if dashboardOrigin == "" {
		dashboardOrigin = "http://localhost:3000"
	}
	passwordHandler := withCORS(dashboardOrigin, auth.PasswordLoginHandler(wos, store))
	mux.Handle("POST /auth/password", passwordHandler)
	mux.HandleFunc("OPTIONS /auth/password", func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w, dashboardOrigin)
		w.WriteHeader(http.StatusNoContent)
	})

	// ── In-process event bus ─────────────────────────────────────────────────
	var bus platformevents.EventBus = inproc.New()

	// ── Redis (idempotency store) ─────────────────────────────────────────────
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	redisClient := redis.NewClient(&redis.Options{Addr: redisAddr})

	// ── Inventory service ─────────────────────────────────────────────────────
	invRepo := inventorypostgres.NewRepository(pool)
	invIdem := inventoryredis.NewIdempotencyStore(redisClient)
	invPublisher := inventoryevents.NewPublisher(bus)
	invSvc := usecases.NewInventoryService(invRepo, invIdem, invPublisher)
	invHandler := inventoryconnect.NewHandler(invSvc)

	// ── Channel service ───────────────────────────────────────────────────────
	connRepo := channelpostgres.NewConnectionRepository(pool)
	chanRepo := channelpostgres.NewChannelRepository(pool)
	syncJobRepo := channelpostgres.NewSyncJobRepository(pool)
	chanSecrets := infra.NewInMemorySecretResolver()
	chanPublisher := channelevents.NewPublisher(bus)
	chanBreaker := infra.NewNoopCircuitBreaker()
	chanSvc := channelusecases.NewChannelService(connRepo, chanRepo, syncJobRepo, chanSecrets, chanPublisher, chanBreaker)

	// Register OTA adapters.
	chanSvc.RegisterAdapter(airbnb.NewAdapter())
	chanSvc.RegisterAdapter(bookingcom.NewAdapter())
	chanSvc.RegisterAdapter(expedia.NewAdapter())
	chanSvc.RegisterAdapter(agoda.NewAdapter())

	connHandler := channelconnect.NewConnectionHandler(chanSvc)
	chanHandler := channelconnect.NewChannelHandler(chanSvc)

	// ── PMS service (MyPMS webhook ingestion) ─────────────────────────────────
	pmsConnRepo := pmspostgres.NewConnectionRepository(pool)
	pmsPropRepo := pmspostgres.NewPropertyRepository(pool)
	pmsRTRepo := pmspostgres.NewRoomTypeRepository(pool)
	pmsRoomRepo := pmspostgres.NewRoomRepository(pool)
	pmsSecrets := pmsinfra.NewInMemorySecretResolver()
	pmsInvWriter := pmsinventory.NewWriter(invSvc)

	// Usecases
	pmsSvc := pmsusecases.NewPmsService(pmsConnRepo, pmsPropRepo, pmsRTRepo, pmsRoomRepo, pmsSecrets, pmsInvWriter)
	pmsHandler := pmsconnect.NewHandler(pmsSvc)

	// ── Reservations service ────────────────────────────────────────────────────
	resRepo := respostgres.NewRepository(pool)
	resSvc := resusecases.NewReservationService(resRepo, resevents.NoopPublisher{})

	// ── Pricing service ────────────────────────────────────────────────────────
	pricingHandler := pricingconnect.NewHandler()

	// ── PMS outbound integration (REST + API key auth) ─────────────────────────
	envSecrets, err := platformintegration.LoadEnvSecretsFromJSON(cfg.Integration.SecretsJSON)
	must(err, "load integration secrets")
	intKeyStore := platformintegration.NewKeyStore(pool)
	intAuth := platformintegration.NewAuthenticator(envSecrets, intKeyStore)
	intSvc := integrationusecases.NewService(pmsPropRepo, chanSvc, syncJobRepo, invSvc, resSvc, pmsSvc)
	intHandler := integrationhttp.NewHandler(intSvc)
	adminKeysHandler := integrationhttp.NewAdminKeysHandler(intKeyStore)

	mux.Handle("GET /api/integrations/pms", intAuth.Middleware(http.HandlerFunc(intHandler.OrgHealth)))
	mux.Handle("POST /api/integrations/pms", intAuth.Middleware(http.HandlerFunc(intHandler.OrgDispatch)))
	mux.Handle("GET /api/integrations/pms/{propertyId}", intAuth.Middleware(http.HandlerFunc(intHandler.PropertyHealth)))
	mux.Handle("POST /api/integrations/pms/{propertyId}", intAuth.Middleware(http.HandlerFunc(intHandler.Dispatch)))

	// ── Connect-RPC interceptor (auth-gated) ──────────────────────────────────
	interceptor := connect.WithInterceptors(auth.NewUnaryInterceptor(enforcer))

	// ── Protected Connect-RPC mux ─────────────────────────────────────────────
	rpcMux := http.NewServeMux()
	rpcPath, rpcHandler := inventoryv1connect.NewInventoryServiceHandler(invHandler, interceptor)
	rpcMux.Handle(rpcPath, rpcHandler)

	connRPCPath, connRPCHandler := channelv1connect.NewConnectionServiceHandler(connHandler, interceptor)
	rpcMux.Handle(connRPCPath, connRPCHandler)

	chanRPCPath, chanRPCHandler := channelv1connect.NewChannelServiceHandler(chanHandler, interceptor)
	rpcMux.Handle(chanRPCPath, chanRPCHandler)

	pmsRPCPath, pmsRPCHandler := pmsv1connect.NewPmsServiceHandler(pmsHandler, interceptor)
	rpcMux.Handle(pmsRPCPath, pmsRPCHandler)

	pricingRPCPath, pricingRPCHandler := pricingv1connect.NewPricingServiceHandler(pricingHandler, interceptor)
	rpcMux.Handle(pricingRPCPath, pricingRPCHandler)

	// ── HTTP mux — public + protected ────────────────────────────────────────
	protected := http.NewServeMux()
	protected.Handle("GET /me", auth.MeHandler(store))
	protected.Handle("PATCH /me/preferences", auth.UpdatePreferencesHandler(store))

	// Team management routes (admin-only, calls WorkOS directly).
	protected.Handle("GET /team/members", auth.ListTeamMembersHandler(wos, store))
	protected.Handle("GET /team/invitations", auth.ListInvitationsHandler(wos, store))
	protected.Handle("POST /team/invite", auth.SendInvitationHandler(wos, store))
	protected.Handle("POST /team/revoke-invitation", auth.RevokeInvitationHandler(wos))
	protected.Handle("POST /team/remove-member", auth.RemoveMemberHandler(wos))

	protected.Handle("GET /admin/integration-keys", http.HandlerFunc(adminKeysHandler.ListKeys))
	protected.Handle("POST /admin/integration-keys", http.HandlerFunc(adminKeysHandler.CreateKey))
	protected.Handle("DELETE /admin/integration-keys/{id}", http.HandlerFunc(adminKeysHandler.RevokeKey))

	protected.Handle(rpcPath, rpcMux)
	protected.Handle(connRPCPath, rpcMux)
	protected.Handle(chanRPCPath, rpcMux)
	protected.Handle(pmsRPCPath, rpcMux)
	protected.Handle(pricingRPCPath, rpcMux)

	mux.Handle("/", auth.NewMiddleware(verifier, store, wos)(protected))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	server := &http.Server{Addr: ":" + port, Handler: mux}

	go func() {
		slog.Info("API server listening", "port", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP server error", "err", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("shutting down...")

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutCancel()
	if err := server.Shutdown(shutCtx); err != nil {
		slog.Error("forced shutdown", "err", err)
		os.Exit(1)
	}

	// Flush OTel batches and Sentry events before exiting.
	if err := observability.Shutdown(shutCtx); err != nil {
		slog.Error("observability shutdown", "err", err)
	}
	slog.Info("API server stopped gracefully")
}

func must(err error, msg string) {
	if err != nil {
		slog.Error(msg, "err", err)
		os.Exit(1)
	}
}

// setCORSHeaders writes the CORS response headers that allow the dashboard
// origin to make credentialed cross-origin requests.
func setCORSHeaders(w http.ResponseWriter, origin string) {
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

// withCORS wraps h so that every response carries the required CORS headers
// for the given origin. Use for endpoints that the dashboard fetches directly.
func withCORS(origin string, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w, origin)
		h.ServeHTTP(w, r)
	})
}
