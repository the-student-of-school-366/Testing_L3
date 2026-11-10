package handler

import (
	"errors"
	"net/http"

	"github.com/anand/api-testing-hw/internal/model"
	"github.com/anand/api-testing-hw/internal/service"
	"github.com/anand/api-testing-hw/internal/storage"
)

// PostHandler serves /api/v1/posts endpoints.
type PostHandler struct {
	posts *service.PostService
}

func NewPostHandler(posts *service.PostService) *PostHandler {
	return &PostHandler{posts: posts}
}

func (h *PostHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.CreatePostRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	post, err := h.posts.Create(req)
	if errors.Is(err, service.ErrValidation) {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create post")
		return
	}
	writeJSON(w, http.StatusCreated, post)
}

func (h *PostHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "post id is required")
		return
	}

	post, err := h.posts.Get(id)
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, "post not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get post")
		return
	}
	writeJSON(w, http.StatusOK, post)
}

func (h *PostHandler) Patch(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "post id is required")
		return
	}

	var req model.PatchPostRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	post, err := h.posts.Patch(id, req)
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, "post not found")
		return
	}
	if errors.Is(err, service.ErrValidation) {
		writeError(w, http.StatusBadRequest, "title cannot be empty")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update post")
		return
	}
	writeJSON(w, http.StatusOK, post)
}
