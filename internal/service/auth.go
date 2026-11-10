package service

import (
	"crypto/rand"
	"encoding/hex"
	"strings"

	"github.com/anand/api-testing-hw/internal/model"
	"github.com/anand/api-testing-hw/internal/storage"
)

var demoUser = model.User{
	ID:       1,
	Name:     "Leanne Graham",
	Username: "bret",
	Email:    "sincere@april.biz",
}

const demoPassword = "Password@123"

// AuthService handles login and token lookup.
type AuthService struct {
	store *storage.Memory
}

func NewAuthService(store *storage.Memory) *AuthService {
	return &AuthService{store: store}
}

func (s *AuthService) Login(username, password string) (model.LoginResponse, error) {
	if !strings.EqualFold(username, demoUser.Username) || password != demoPassword {
		return model.LoginResponse{}, storage.ErrInvalidCreds
	}

	token, err := randomToken()
	if err != nil {
		return model.LoginResponse{}, err
	}
	s.store.SaveToken(token, demoUser)

	return model.LoginResponse{
		AccessToken: token,
		TokenType:   "Bearer",
		User:        demoUser,
	}, nil
}

func (s *AuthService) Me(token string) (model.User, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return model.User{}, storage.ErrUnauthorized
	}
	return s.store.UserByToken(token)
}

func randomToken() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
