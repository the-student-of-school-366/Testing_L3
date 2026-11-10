package handler

import (
	"net/http"

	"github.com/anand/api-testing-hw/internal/middleware"
	"github.com/anand/api-testing-hw/internal/service"
)

// Dependencies groups services used by HTTP handlers.
type Dependencies struct {
	Posts *service.PostService
	Auth  *service.AuthService
}

// NewRouter builds the application HTTP mux.
func NewRouter(deps Dependencies) http.Handler {
	posts := NewPostHandler(deps.Posts)
	auth := NewAuthHandler(deps.Auth)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", Health)
	mux.HandleFunc("POST /api/v1/posts", posts.Create)
	mux.HandleFunc("GET /api/v1/posts/{id}", posts.Get)
	mux.HandleFunc("PATCH /api/v1/posts/{id}", posts.Patch)
	mux.HandleFunc("POST /api/v1/auth/login", auth.Login)
	mux.HandleFunc("GET /api/v1/auth/me", auth.Me)

	return middleware.Chain(mux, middleware.RequestID, middleware.Logging, middleware.JSONContentType)
}
