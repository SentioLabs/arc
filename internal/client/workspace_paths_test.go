package client

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListWorkspacePaths(t *testing.T) {
	expected := []*WorkspacePath{
		{ID: "p-1", WorkspaceID: "ws-abc", Path: "/home/user/project"},
		{ID: "p-2", WorkspaceID: "ws-abc", Path: "/tmp/worktree"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/workspaces/ws-abc/paths" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expected)
	}))
	defer server.Close()

	c := New(server.URL)
	paths, err := c.ListWorkspacePaths("ws-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 2 {
		t.Fatalf("expected 2 paths, got %d", len(paths))
	}
	if paths[0].ID != "p-1" {
		t.Errorf("expected ID p-1, got %s", paths[0].ID)
	}
	if paths[1].Path != "/tmp/worktree" {
		t.Errorf("expected path /tmp/worktree, got %s", paths[1].Path)
	}
}

func TestCreateWorkspacePath(t *testing.T) {
	expected := &WorkspacePath{
		ID:          "p-new",
		WorkspaceID: "ws-abc",
		Path:        "/home/user/project",
		Label:       "main",
		Hostname:    "dev-machine",
		GitRemote:   "git@github.com:user/project.git",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/workspaces/ws-abc/paths" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		body, _ := io.ReadAll(r.Body)
		var req CreateWorkspacePathRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if req.Path != "/home/user/project" {
			t.Errorf("expected path /home/user/project, got %s", req.Path)
		}
		if req.Label != "main" {
			t.Errorf("expected label main, got %s", req.Label)
		}
		if req.Hostname != "dev-machine" {
			t.Errorf("expected hostname dev-machine, got %s", req.Hostname)
		}
		if req.GitRemote != "git@github.com:user/project.git" {
			t.Errorf("expected git_remote, got %s", req.GitRemote)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(expected)
	}))
	defer server.Close()

	c := New(server.URL)
	result, err := c.CreateWorkspacePath("ws-abc", CreateWorkspacePathRequest{
		Path:      "/home/user/project",
		Label:     "main",
		Hostname:  "dev-machine",
		GitRemote: "git@github.com:user/project.git",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != "p-new" {
		t.Errorf("expected ID p-new, got %s", result.ID)
	}
	if result.Label != "main" {
		t.Errorf("expected label main, got %s", result.Label)
	}
}

func TestDeleteWorkspacePath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/workspaces/ws-abc/paths/p-1" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	c := New(server.URL)
	err := c.DeleteWorkspacePath("ws-abc", "p-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveWorkspaceByPath(t *testing.T) {
	expected := &WorkspaceResolution{
		WorkspaceID:   "ws-abc",
		WorkspaceName: "my-project",
		PathID:        "p-1",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/workspaces/resolve" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		queryPath := r.URL.Query().Get("path")
		if queryPath != "/home/user/project" {
			t.Errorf("expected query path /home/user/project, got %s", queryPath)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expected)
	}))
	defer server.Close()

	c := New(server.URL)
	result, err := c.ResolveWorkspaceByPath("/home/user/project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.WorkspaceID != "ws-abc" {
		t.Errorf("expected workspace ID ws-abc, got %s", result.WorkspaceID)
	}
	if result.WorkspaceName != "my-project" {
		t.Errorf("expected workspace name my-project, got %s", result.WorkspaceName)
	}
	if result.PathID != "p-1" {
		t.Errorf("expected path ID p-1, got %s", result.PathID)
	}
}

func TestUpdateWorkspacePath(t *testing.T) {
	expected := &WorkspacePath{
		ID:          "p-1",
		WorkspaceID: "ws-abc",
		Path:        "/home/user/project",
		Label:       "updated-label",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" {
			t.Errorf("expected PATCH, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/workspaces/ws-abc/paths/p-1" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		body, _ := io.ReadAll(r.Body)
		var updates map[string]string
		if err := json.Unmarshal(body, &updates); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if updates["label"] != "updated-label" {
			t.Errorf("expected label updated-label, got %s", updates["label"])
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expected)
	}))
	defer server.Close()

	c := New(server.URL)
	result, err := c.UpdateWorkspacePath("ws-abc", "p-1", map[string]string{"label": "updated-label"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != "p-1" {
		t.Errorf("expected ID p-1, got %s", result.ID)
	}
	if result.Label != "updated-label" {
		t.Errorf("expected label updated-label, got %s", result.Label)
	}
}
