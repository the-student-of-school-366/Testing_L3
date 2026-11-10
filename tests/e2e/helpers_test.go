package e2e_test

import (
	"os"
	"testing"

	"github.com/anand/api-testing-hw/internal/client"
)

const (
	playgroundBaseURL = "https://playground.nileslabs.com/api/v1"
	graphqlEchoURL    = "https://graphql.postman-echo.com/graphql"

	loginUsername = "Bret"
	loginPassword = "Password@123"
)

func newPlaygroundClient(t *testing.T) *client.Client {
	t.Helper()
	c := client.New(playgroundBaseURL)
	c.Identity = playgroundIdentity()
	return c
}

func newGraphQLClient(t *testing.T) *client.Client {
	t.Helper()
	return client.New(graphqlEchoURL)
}

func playgroundIdentity() string {
	if v := os.Getenv("PLAYGROUND_IDENTITY"); v != "" {
		return v
	}
	h, err := os.Hostname()
	if err != nil || h == "" {
		h = "local"
	}
	return "go-api-hw-" + h
}

func doJSON(t *testing.T, c *client.Client, method, path string, headers map[string]string, body any) *client.Response {
	t.Helper()
	resp, err := c.DoJSON(method, path, headers, body)
	if err != nil {
		t.Fatalf("http request failed: %v", err)
	}
	return resp
}

func requireStatus(t *testing.T, resp *client.Response, want int) {
	t.Helper()
	if resp.StatusCode != want {
		t.Log(resp.Diagnostics())
		t.Fatalf("unexpected status: got %d, want %d", resp.StatusCode, want)
	}
}

func decodeJSON(t *testing.T, resp *client.Response, dest any) {
	t.Helper()
	if err := resp.Decode(dest); err != nil {
		t.Log(resp.Diagnostics())
		t.Fatalf("decode JSON: %v", err)
	}
}

func failWithDiag(t *testing.T, resp *client.Response, format string, args ...any) {
	t.Helper()
	t.Log(resp.Diagnostics())
	t.Fatalf(format, args...)
}
