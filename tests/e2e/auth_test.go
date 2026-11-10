package e2e_test

import (
	"strings"
	"testing"

	"github.com/anand/api-testing-hw/internal/client"
)

func TestAuth_LoginThenMe(t *testing.T) {
	c := newPlaygroundClient(t)

	loginResp := doJSON(t, c, "POST", "/auth/login", nil, map[string]string{
		"username": loginUsername,
		"password": loginPassword,
	})
	requireStatus(t, loginResp, 200)

	var login client.LoginResponse
	decodeJSON(t, loginResp, &login)

	if strings.TrimSpace(login.AccessToken) == "" {
		failWithDiag(t, loginResp, "access_token must be a non-empty string")
	}
	if login.User.Username == "" {
		failWithDiag(t, loginResp, "login response must include user.username")
	}

	token := login.AccessToken

	meResp := doJSON(t, c, "GET", "/auth/me", map[string]string{
		"Authorization": "Bearer " + token,
	}, nil)
	requireStatus(t, meResp, 200)

	var me client.User
	decodeJSON(t, meResp, &me)

	if me.Username == "" {
		failWithDiag(t, meResp, "expected username in /auth/me response")
	}
	if !strings.EqualFold(me.Username, loginUsername) {
		failWithDiag(t, meResp, "/auth/me username: got %q, want %q", me.Username, loginUsername)
	}
	if me.Email == "" {
		failWithDiag(t, meResp, "expected email identifying field in /auth/me")
	}
	if me.ID == nil || client.AsString(me.ID) == "" {
		failWithDiag(t, meResp, "expected id identifying field in /auth/me")
	}
}

func TestAuth_MeWithInvalidToken(t *testing.T) {
	c := newPlaygroundClient(t)

	cases := []struct {
		name   string
		header map[string]string
	}{
		{name: "missing_authorization", header: nil},
		{name: "wrong_bearer_token", header: map[string]string{"Authorization": "Bearer definitely-not-a-valid-token"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := doJSON(t, c, "GET", "/auth/me", tc.header, nil)
			if resp.StatusCode == 200 {
				failWithDiag(t, resp, "expected access denial, got HTTP 200")
			}
			if resp.StatusCode != 401 && resp.StatusCode != 403 {
				failWithDiag(t, resp, "expected 401/403 for invalid auth, got %d", resp.StatusCode)
			}

			var errBody map[string]any
			decodeJSON(t, resp, &errBody)
			if errBody["error"] == nil && errBody["message"] == nil {
				failWithDiag(t, resp, "expected error payload in denied /auth/me response")
			}
		})
	}
}
