package observability

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/getsentry/sentry-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Config holds the configuration for observability tooling.
type Config struct {
	ServiceName  string
	OTLPEndpoint string // host:port, e.g. "localhost:4317"
	SentryDSN    string
}

// package-level state — set by Init, consumed by Shutdown.
var (
	mu           sync.Mutex
	shutdownFns  []func(context.Context) error
	sentryLoaded bool
)

// Init initializes OpenTelemetry (traces + metrics) and Sentry.
// It sets the global TracerProvider, MeterProvider, and TextMapPropagator so
// that all instrumented code in the process picks them up automatically.
//
// Call Shutdown on process exit to flush buffered telemetry.
func Init(cfg Config) error {
	mu.Lock()
	defer mu.Unlock()

	// ── Resource (service identity attached to every span / data-point) ───────
	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String(cfg.ServiceName),
		),
		resource.WithTelemetrySDK(),
		resource.WithProcess(),
	)
	if err != nil {
		return wrapErr("build otel resource", err)
	}

	// ── gRPC connection to the OTLP collector ─────────────────────────────────
	conn, err := grpc.NewClient(
		cfg.OTLPEndpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return wrapErr("dial OTLP endpoint", err)
	}

	// ── Trace exporter + provider ─────────────────────────────────────────────
	traceExp, err := otlptracegrpc.New(context.Background(),
		otlptracegrpc.WithGRPCConn(conn),
	)
	if err != nil {
		return wrapErr("create trace exporter", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExp),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(tp)
	shutdownFns = append(shutdownFns, tp.Shutdown)

	// ── Metric exporter + provider ────────────────────────────────────────────
	metricExp, err := otlpmetricgrpc.New(context.Background(),
		otlpmetricgrpc.WithGRPCConn(conn),
	)
	if err != nil {
		return wrapErr("create metric exporter", err)
	}

	mp := metric.NewMeterProvider(
		metric.WithReader(metric.NewPeriodicReader(metricExp,
			metric.WithInterval(15*time.Second),
		)),
		metric.WithResource(res),
	)
	otel.SetMeterProvider(mp)
	shutdownFns = append(shutdownFns, mp.Shutdown)

	// ── W3C TraceContext + Baggage propagation ────────────────────────────────
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// ── Sentry (optional — skipped when DSN is empty) ─────────────────────────
	if cfg.SentryDSN != "" {
		if err := sentry.Init(sentry.ClientOptions{
			Dsn:              cfg.SentryDSN,
			EnableTracing:    true,
			TracesSampleRate: 1.0,
		}); err != nil {
			return wrapErr("init sentry", err)
		}
		sentryLoaded = true
	}

	return nil
}

// Shutdown flushes and closes all observability providers.
// Pass a context with a deadline to bound the flush time.
func Shutdown(ctx context.Context) error {
	mu.Lock()
	defer mu.Unlock()

	var errs []error

	for _, fn := range shutdownFns {
		if err := fn(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	shutdownFns = nil

	if sentryLoaded {
		sentry.Flush(2 * time.Second)
		sentryLoaded = false
	}

	return errors.Join(errs...)
}

func wrapErr(op string, err error) error {
	if err == nil {
		return nil
	}
	return errors.New("observability: " + op + ": " + err.Error())
}
