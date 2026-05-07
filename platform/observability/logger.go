package observability

import (
	"log/slog"
	"os"
)

// NewLogger creates a new structured logger using slog with JSON output.
func NewLogger(serviceName string) *slog.Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	return slog.New(handler).With("service", serviceName)
}
