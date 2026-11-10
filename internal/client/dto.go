package client

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// Post is a post resource DTO (compatible with Playground API).
type Post struct {
	ID     any    `json:"id"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	UserID any    `json:"user_id"`
}

// LoginResponse is returned by POST /auth/login.
type LoginResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	User        User   `json:"user"`
}

// User is a user profile DTO.
type User struct {
	ID       any    `json:"id"`
	Name     string `json:"name"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

// GraphQLResponse is a GraphQL envelope.
type GraphQLResponse struct {
	Data   map[string]any `json:"data"`
	Errors []GraphQLError `json:"errors"`
}

// GraphQLError is a single GraphQL error.
type GraphQLError struct {
	Message string `json:"message"`
}

// AsString converts JSON scalar values to string for assertions.
func AsString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		if x == float64(int64(x)) {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	case json.Number:
		return x.String()
	case nil:
		return ""
	default:
		return fmt.Sprint(x)
	}
}
