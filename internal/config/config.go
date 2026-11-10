package config

import (
	"os"
	"time"
)

// Config holds runtime settings for the HTTP server.
type Config struct {
	Addr            string
	ShutdownTimeout time.Duration
}

// Load reads configuration from environment with sensible defaults.
func Load() Config {
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	return Config{
		Addr:            addr,
		ShutdownTimeout: 10 * time.Second,
	}
}
