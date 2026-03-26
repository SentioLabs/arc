package client_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sentiolabs/arc/internal/client"
	"github.com/sentiolabs/arc/internal/types"
)

const (
	testProjectID       = "p-1"
	testUserProjectPath = "/home/user/project"
)

func TestListWorkspaces(t *testing.T) {
	expected := []*types.Workspace{
		{ID: testProjectID, ProjectID: "proj-abc", Path: testUserProjectPath},
		{ID: "p-2", ProjectID: "proj-abc", Path: "/tmp/worktree"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/projects/proj-abc/workspaces" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(expected)
	}))
	defer server.Close()

	c := client.New(server.URL)
	paths, err := c.ListWorkspaces("proj-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 2 {
		t.Fatalf("expected 2 paths, got %d", len(paths))
	}
	if paths[0].ID != testProjectID {
		t.Errorf("expected ID p-1, got %s", paths[0].ID)
	}
	if paths[1].Path != "/tmp/worktree" {
		t.Errorf("expected path /tmp/worktree, got %s", paths[1].Path)
	}
}

func TestCreateWorkspace(t *testing.T) {
	expected := &types.Workspace{
		ID:        "p-new",
		ProjectID: "proj-abc",
		Path:      testUserProjectPath,
		Label:     "main",
		Hostname:  "dev-machine",
		GitRemote: "git@github.com:user/project.git",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/projects/proj-abc/workspaces" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		body, _ := io.ReadAll(r.Body)
		var req client.CreateWorkspaceRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if req.Path != testUserProjectPath {
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
		_ = json.NewEncoder(w).Encode(expected)
	}))
	defer server.Close()

	c := client.New(server.URL)
	result, err := c.CreateWorkspace("proj-abc", client.CreateWorkspaceRequest{
		Path:      testUserProjectPath,
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

func TestDeleteWorkspace(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/projects/proj-abc/workspaces/p-1" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	c := client.New(server.URL)
	err := c.DeleteWorkspace("proj-abc", testProjectID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveProjectByPath(t *testing.T) {
	expected := &types.ProjectResolution{
		ProjectID:   "proj-abc",
		ProjectName: "my-project",
		PathID:      testProjectID,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/projects/resolve" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		queryPath := r.URL.Query().Get("path")
		if queryPath != testUserProjectPath {
			t.Errorf("expected query path /home/user/project, got %s", queryPath)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(expected)
	}))
	defer server.Close()

	c := client.New(server.URL)
	result, err := c.ResolveProjectByPath(testUserProjectPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ProjectID != "proj-abc" {
		t.Errorf("expected project ID proj-abc, got %s", result.ProjectID)
	}
	if result.ProjectName != "my-project" {
		t.Errorf("expected project name my-project, got %s", result.ProjectName)
	}
	if result.PathID != testProjectID {
		t.Errorf("expected path ID p-1, got %s", result.PathID)
	}
}

func TestUpdateWorkspace(t *testing.T) {
	expected := &types.Workspace{
		ID:        testProjectID,
		ProjectID: "proj-abc",
		Path:      testUserProjectPath,
		Label:     "updated-label",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" {
			t.Errorf("expected PATCH, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/projects/proj-abc/workspaces/p-1" {
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
		_ = json.NewEncoder(w).Encode(expected)
	}))
	defer server.Close()

	c := client.New(server.URL)
	result, err := c.UpdateWorkspace("proj-abc", testProjectID, map[string]string{"label": "updated-label"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != testProjectID {
		t.Errorf("expected ID p-1, got %s", result.ID)
	}
	if result.Label != "updated-label" {
		t.Errorf("expected label updated-label, got %s", result.Label)
	}
}
