package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/anand/api-testing-hw/internal/app"
	"github.com/anand/api-testing-hw/internal/config"
)

func TestLocalAPI_CreatePatchGetAndAuth(t *testing.T) {
	application := app.New(config.Config{Addr: ":0"})
	server := httptest.NewServer(application.Handler())
	t.Cleanup(server.Close)

	// Create post
	createBody := `{"title":"hello","body":"world","user_id":1}`
	createResp := postJSON(t, server.URL+"/api/v1/posts", createBody, nil)
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("create status: got %d", createResp.StatusCode)
	}
	var created map[string]any
	decode(t, createResp, &created)
	id, _ := created["id"].(string)
	if id == "" {
		t.Fatal("expected post id")
	}

	// Patch title
	patchResp := patchJSON(t, server.URL+"/api/v1/posts/"+id, `{"title":"updated"}`, nil)
	if patchResp.StatusCode != http.StatusOK {
		t.Fatalf("patch status: got %d", patchResp.StatusCode)
	}

	// Get and verify
	getResp := getJSON(t, server.URL+"/api/v1/posts/"+id, nil)
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("get status: got %d", getResp.StatusCode)
	}
	var got map[string]any
	decode(t, getResp, &got)
	if got["title"] != "updated" {
		t.Fatalf("title: got %#v", got["title"])
	}
	if got["body"] != "world" {
		t.Fatalf("body should be unchanged, got %#v", got["body"])
	}

	// Login
	loginResp := postJSON(t, server.URL+"/api/v1/auth/login", `{"username":"Bret","password":"Password@123"}`, nil)
	if loginResp.StatusCode != http.StatusOK {
		t.Fatalf("login status: got %d", loginResp.StatusCode)
	}
	var login map[string]any
	decode(t, loginResp, &login)
	token, _ := login["access_token"].(string)
	if token == "" {
		t.Fatal("expected access_token")
	}

	// Me
	meResp := getJSON(t, server.URL+"/api/v1/auth/me", map[string]string{
		"Authorization": "Bearer " + token,
	})
	if meResp.StatusCode != http.StatusOK {
		t.Fatalf("me status: got %d", meResp.StatusCode)
	}

	// Negative me
	badMe := getJSON(t, server.URL+"/api/v1/auth/me", nil)
	if badMe.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", badMe.StatusCode)
	}
}

func postJSON(t *testing.T, url, body string, headers map[string]string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func patchJSON(t *testing.T, url, body string, headers map[string]string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPatch, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func getJSON(t *testing.T, url string, headers map[string]string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func decode(t *testing.T, resp *http.Response, dest any) {
	t.Helper()
	defer resp.Body.Close()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(buf.Bytes(), dest); err != nil {
		t.Fatalf("decode: %v body=%s", err, buf.String())
	}
}
