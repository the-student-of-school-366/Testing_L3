package e2e_test

import (
	"strings"
	"testing"
)

func TestAuth_LoginRejectedCredentials(t *testing.T) {
	c := newPlaygroundClient(t)

	cases := []struct {
		name     string
		username string
		password string
	}{
		{name: "wrong_password", username: loginUsername, password: "definitely-wrong-password"},
		{name: "unknown_user", username: "no_such_user_xyz", password: loginPassword},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := doJSON(t, c, "POST", "/auth/login", nil, map[string]string{
				"username": tc.username,
				"password": tc.password,
			})

			if resp.StatusCode == 200 {
				failWithDiag(t, resp, "expected login rejection, got HTTP 200")
			}
			if resp.StatusCode != 401 && resp.StatusCode != 403 {
				failWithDiag(t, resp, "expected 401/403 for bad credentials, got %d", resp.StatusCode)
			}

			var body map[string]any
			decodeJSON(t, resp, &body)
			if token, ok := body["access_token"].(string); ok && strings.TrimSpace(token) != "" {
				failWithDiag(t, resp, "access_token must not be issued for rejected login")
			}
			if body["error"] == nil && body["message"] == nil {
				failWithDiag(t, resp, "expected error/message in rejected login response")
			}
		})
	}
}

func TestREST_GetUnknownPost(t *testing.T) {
	c := newPlaygroundClient(t)

	resp := doJSON(t, c, "GET", "/posts/local-does-not-exist-0000", nil, nil)
	if resp.StatusCode != 404 {
		failWithDiag(t, resp, "expected HTTP 404 for unknown post, got %d", resp.StatusCode)
	}

	var body map[string]any
	decodeJSON(t, resp, &body)
	if body["error"] == nil && body["message"] == nil {
		failWithDiag(t, resp, "expected error payload for unknown post")
	}
}
