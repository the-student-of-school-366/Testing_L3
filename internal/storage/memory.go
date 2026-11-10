package storage

import (
	"errors"
	"sync"

	"github.com/anand/api-testing-hw/internal/model"
)

var (
	ErrNotFound     = errors.New("not found")
	ErrUnauthorized = errors.New("unauthorized")
	ErrInvalidCreds = errors.New("invalid credentials")
)

// Memory is an in-memory store for posts and issued tokens.
type Memory struct {
	mu     sync.RWMutex
	posts  map[string]model.Post
	tokens map[string]model.User
}

// NewMemory creates an empty in-memory store.
func NewMemory() *Memory {
	return &Memory{
		posts:  make(map[string]model.Post),
		tokens: make(map[string]model.User),
	}
}

func (m *Memory) SavePost(post model.Post) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.posts[post.ID] = post
}

func (m *Memory) GetPost(id string) (model.Post, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	post, ok := m.posts[id]
	if !ok {
		return model.Post{}, ErrNotFound
	}
	return post, nil
}

func (m *Memory) UpdatePost(post model.Post) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.posts[post.ID]; !ok {
		return ErrNotFound
	}
	m.posts[post.ID] = post
	return nil
}

func (m *Memory) SaveToken(token string, user model.User) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tokens[token] = user
}

func (m *Memory) UserByToken(token string) (model.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	user, ok := m.tokens[token]
	if !ok {
		return model.User{}, ErrUnauthorized
	}
	return user, nil
}
