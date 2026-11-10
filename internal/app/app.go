package app

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/anand/api-testing-hw/internal/config"
	"github.com/anand/api-testing-hw/internal/handler"
	"github.com/anand/api-testing-hw/internal/service"
	"github.com/anand/api-testing-hw/internal/storage"
)

// App wires dependencies and owns the HTTP server lifecycle.
type App struct {
	cfg    config.Config
	server *http.Server
}

// New builds the application from config.
func New(cfg config.Config) *App {
	store := storage.NewMemory()
	posts := service.NewPostService(store)
	auth := service.NewAuthService(store)

	router := handler.NewRouter(handler.Dependencies{
		Posts: posts,
		Auth:  auth,
	})

	return &App{
		cfg: cfg,
		server: &http.Server{
			Addr:              cfg.Addr,
			Handler:           router,
			ReadHeaderTimeout: 5 * time.Second,
		},
	}
}

// Handler exposes the HTTP handler (useful for tests).
func (a *App) Handler() http.Handler {
	return a.server.Handler
}

// Run starts the HTTP server and blocks until it stops.
func (a *App) Run() error {
	log.Printf("listening on %s", a.cfg.Addr)
	if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("listen: %w", err)
	}
	return nil
}

// Shutdown gracefully stops the server.
func (a *App) Shutdown(ctx context.Context) error {
	return a.server.Shutdown(ctx)
}
