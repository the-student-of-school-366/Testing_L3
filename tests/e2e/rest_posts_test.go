package e2e_test

import (
	"testing"

	"github.com/anand/api-testing-hw/internal/client"
)

// TestREST_CreatePatchGetPost: POST → extract id → PATCH → GET.
func TestREST_CreatePatchGetPost(t *testing.T) {
	c := newPlaygroundClient(t)

	originalTitle := "Go HW original title"
	originalBody := "Body that must survive PATCH"
	updatedTitle := "Go HW updated title"

	createResp := doJSON(t, c, "POST", "/posts", nil, map[string]any{
		"title":   originalTitle,
		"body":    originalBody,
		"user_id": 1,
	})
	requireStatus(t, createResp, 201)

	var created client.Post
	decodeJSON(t, createResp, &created)
	if created.ID == nil || client.AsString(created.ID) == "" {
		failWithDiag(t, createResp, "expected non-empty id in POST /posts response")
	}
	if created.Title != originalTitle {
		failWithDiag(t, createResp, "created title: got %q, want %q", created.Title, originalTitle)
	}
	if created.Body != originalBody {
		failWithDiag(t, createResp, "created body: got %q, want %q", created.Body, originalBody)
	}

	postID := client.AsString(created.ID)
	t.Logf("created post id=%s (dynamic, not hardcoded)", postID)

	patchResp := doJSON(t, c, "PATCH", "/posts/"+postID, nil, map[string]any{
		"title": updatedTitle,
	})
	requireStatus(t, patchResp, 200)

	var patched client.Post
	decodeJSON(t, patchResp, &patched)
	if client.AsString(patched.ID) != postID {
		failWithDiag(t, patchResp, "patched id: got %q, want %q", client.AsString(patched.ID), postID)
	}
	if patched.Title != updatedTitle {
		failWithDiag(t, patchResp, "patched title: got %q, want %q", patched.Title, updatedTitle)
	}

	getResp := doJSON(t, c, "GET", "/posts/"+postID, nil, nil)
	requireStatus(t, getResp, 200)

	var got client.Post
	decodeJSON(t, getResp, &got)
	if client.AsString(got.ID) != postID {
		failWithDiag(t, getResp, "GET id: got %q, want %q", client.AsString(got.ID), postID)
	}
	if got.Title != updatedTitle {
		failWithDiag(t, getResp, "GET title after PATCH: got %q, want %q", got.Title, updatedTitle)
	}
	if got.Body != originalBody {
		failWithDiag(t, getResp, "unchanged field body: got %q, want %q", got.Body, originalBody)
	}
	if client.AsString(got.UserID) != "1" {
		failWithDiag(t, getResp, "unchanged field user_id: got %q, want 1", client.AsString(got.UserID))
	}
}
