package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/anand/api-testing-hw/internal/model"
	"github.com/anand/api-testing-hw/internal/service"
	"github.com/anand/api-testing-hw/internal/storage"
)

// AuthHandler serves /api/v1/auth endpoints.
type AuthHandler struct {
	auth *service.AuthService
}

func NewAuthHandler(auth *service.AuthService) *AuthHandler {
	return &AuthHandler{auth: auth}
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req model.LoginRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	resp, err := h.auth.Login(req.Username, req.Password)
	if errors.Is(err, storage.ErrInvalidCreds) {
		writeError(w, http.StatusUnauthorized, "Invalid username or password.")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "login failed")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	token := bearerToken(r.Header.Get("Authorization"))
	user, err := h.auth.Me(token)
	if errors.Is(err, storage.ErrUnauthorized) {
		writeError(w, http.StatusUnauthorized, "Unauthorized. Please provide a valid Bearer access token in the Authorization header.")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load profile")
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func bearerToken(header string) string {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(header, prefix))
}
