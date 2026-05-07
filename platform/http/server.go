package http

import (
	"fmt"
	"net/http"
	"time"
)

// ServerConfig holds the configuration for the HTTP server.
type ServerConfig struct {
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// NewServer creates a new http.Server with the given configuration.
func NewServer(cfg ServerConfig) *http.Server {
	return &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}
}
