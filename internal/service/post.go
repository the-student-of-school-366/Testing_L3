package service

import (
	"crypto/rand"
	"encoding/hex"
	"strings"

	"github.com/anand/api-testing-hw/internal/model"
	"github.com/anand/api-testing-hw/internal/storage"
)

// PostService manages post CRUD operations.
type PostService struct {
	store *storage.Memory
}

func NewPostService(store *storage.Memory) *PostService {
	return &PostService{store: store}
}

func (s *PostService) Create(req model.CreatePostRequest) (model.Post, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return model.Post{}, ErrValidation
	}
	if req.UserID == 0 {
		req.UserID = 1
	}

	id, err := randomID("post")
	if err != nil {
		return model.Post{}, err
	}

	post := model.Post{
		ID:     id,
		Title:  title,
		Body:   req.Body,
		UserID: req.UserID,
	}
	s.store.SavePost(post)
	return post, nil
}

func (s *PostService) Patch(id string, req model.PatchPostRequest) (model.Post, error) {
	post, err := s.store.GetPost(id)
	if err != nil {
		return model.Post{}, err
	}
	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" {
			return model.Post{}, ErrValidation
		}
		post.Title = title
	}
	if req.Body != nil {
		post.Body = *req.Body
	}
	if err := s.store.UpdatePost(post); err != nil {
		return model.Post{}, err
	}
	return post, nil
}

func (s *PostService) Get(id string) (model.Post, error) {
	return s.store.GetPost(id)
}

func randomID(prefix string) (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return prefix + "-" + hex.EncodeToString(buf), nil
}
