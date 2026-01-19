// Package client provides an API client for the arc server.
package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/sentiolabs/arc/internal/types"
)

// Client is the API client for arc.
type Client struct {
	baseURL    string
	httpClient *http.Client
	actor      string
}

// New creates a new API client.
func New(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		actor: "cli",
	}
}

// SetActor sets the actor for API requests.
func (c *Client) SetActor(actor string) {
	c.actor = actor
}

// Health checks the server health.
func (c *Client) Health() error {
	resp, err := c.get("/health")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server unhealthy: %d", resp.StatusCode)
	}
	return nil
}

// Workspace methods

// ListWorkspaces returns all workspaces.
func (c *Client) ListWorkspaces() ([]*types.Workspace, error) {
	resp, err := c.get("/api/v1/workspaces")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var workspaces []*types.Workspace
	if err := json.NewDecoder(resp.Body).Decode(&workspaces); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return workspaces, nil
}

// CreateWorkspace creates a new workspace.
func (c *Client) CreateWorkspace(name, prefix, path, description string) (*types.Workspace, error) {
	body := map[string]string{
		"name":        name,
		"prefix":      prefix,
		"path":        path,
		"description": description,
	}

	resp, err := c.post("/api/v1/workspaces", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var ws types.Workspace
	if err := json.NewDecoder(resp.Body).Decode(&ws); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &ws, nil
}

// GetWorkspace retrieves a workspace by ID.
func (c *Client) GetWorkspace(id string) (*types.Workspace, error) {
	resp, err := c.get("/api/v1/workspaces/" + id)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var ws types.Workspace
	if err := json.NewDecoder(resp.Body).Decode(&ws); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &ws, nil
}

// DeleteWorkspace deletes a workspace.
func (c *Client) DeleteWorkspace(id string) error {
	resp, err := c.delete("/api/v1/workspaces/" + id)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// Issue methods

// ListIssues returns issues for a workspace.
func (c *Client) ListIssues(wsID string, opts ListIssuesOptions) ([]*types.Issue, error) {
	path := fmt.Sprintf("/api/v1/workspaces/%s/issues", wsID)

	query := url.Values{}
	if opts.Status != "" {
		query.Set("status", opts.Status)
	}
	if opts.Type != "" {
		query.Set("type", opts.Type)
	}
	if opts.Assignee != "" {
		query.Set("assignee", opts.Assignee)
	}
	if opts.Query != "" {
		query.Set("q", opts.Query)
	}
	if opts.Limit > 0 {
		query.Set("limit", fmt.Sprintf("%d", opts.Limit))
	}

	if len(query) > 0 {
		path += "?" + query.Encode()
	}

	resp, err := c.get(path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data []*types.Issue `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return result.Data, nil
}

// ListIssuesOptions configures issue listing.
type ListIssuesOptions struct {
	Status   string
	Type     string
	Assignee string
	Query    string
	Limit    int
}

// CreateIssue creates a new issue.
func (c *Client) CreateIssue(wsID string, req CreateIssueRequest) (*types.Issue, error) {
	path := fmt.Sprintf("/api/v1/workspaces/%s/issues", wsID)

	resp, err := c.post(path, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var issue types.Issue
	if err := json.NewDecoder(resp.Body).Decode(&issue); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &issue, nil
}

// CreateIssueRequest is the request for creating an issue.
type CreateIssueRequest struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Priority    int    `json:"priority,omitempty"`
	IssueType   string `json:"issue_type,omitempty"`
	Assignee    string `json:"assignee,omitempty"`
	ParentID    string `json:"parent_id,omitempty"` // For hierarchical child IDs
}

// GetIssue retrieves an issue by ID.
func (c *Client) GetIssue(wsID, id string) (*types.Issue, error) {
	path := fmt.Sprintf("/api/v1/workspaces/%s/issues/%s", wsID, id)

	resp, err := c.get(path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var issue types.Issue
	if err := json.NewDecoder(resp.Body).Decode(&issue); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &issue, nil
}

// GetIssueDetails retrieves an issue with all relational data.
func (c *Client) GetIssueDetails(wsID, id string) (*types.IssueDetails, error) {
	path := fmt.Sprintf("/api/v1/workspaces/%s/issues/%s?details=true", wsID, id)

	resp, err := c.get(path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var details types.IssueDetails
	if err := json.NewDecoder(resp.Body).Decode(&details); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &details, nil
}

// UpdateIssue updates an issue.
func (c *Client) UpdateIssue(wsID, id string, updates map[string]interface{}) (*types.Issue, error) {
	path := fmt.Sprintf("/api/v1/workspaces/%s/issues/%s", wsID, id)

	resp, err := c.put(path, updates)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var issue types.Issue
	if err := json.NewDecoder(resp.Body).Decode(&issue); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &issue, nil
}

// CloseIssue closes an issue.
func (c *Client) CloseIssue(wsID, id, reason string) (*types.Issue, error) {
	path := fmt.Sprintf("/api/v1/workspaces/%s/issues/%s/close", wsID, id)

	body := map[string]string{"reason": reason}
	resp, err := c.post(path, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var issue types.Issue
	if err := json.NewDecoder(resp.Body).Decode(&issue); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &issue, nil
}

// DeleteIssue deletes an issue.
func (c *Client) DeleteIssue(wsID, id string) error {
	path := fmt.Sprintf("/api/v1/workspaces/%s/issues/%s", wsID, id)

	resp, err := c.delete(path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// Ready work methods

// GetReadyWork returns issues ready to work on.
// sortPolicy can be: "hybrid" (default), "priority", or "oldest".
func (c *Client) GetReadyWork(wsID string, limit int, sortPolicy string) ([]*types.Issue, error) {
	path := fmt.Sprintf("/api/v1/workspaces/%s/ready", wsID)

	query := url.Values{}
	if limit > 0 {
		query.Set("limit", fmt.Sprintf("%d", limit))
	}
	if sortPolicy != "" {
		query.Set("sort", sortPolicy)
	}
	if len(query) > 0 {
		path += "?" + query.Encode()
	}

	resp, err := c.get(path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var issues []*types.Issue
	if err := json.NewDecoder(resp.Body).Decode(&issues); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return issues, nil
}

// GetBlockedIssues returns blocked issues.
func (c *Client) GetBlockedIssues(wsID string, limit int) ([]*types.BlockedIssue, error) {
	path := fmt.Sprintf("/api/v1/workspaces/%s/blocked", wsID)
	if limit > 0 {
		path += fmt.Sprintf("?limit=%d", limit)
	}

	resp, err := c.get(path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var issues []*types.BlockedIssue
	if err := json.NewDecoder(resp.Body).Decode(&issues); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return issues, nil
}

// Dependency methods

// AddDependency adds a dependency.
func (c *Client) AddDependency(wsID, issueID, dependsOnID, depType string) error {
	path := fmt.Sprintf("/api/v1/workspaces/%s/issues/%s/deps", wsID, issueID)

	body := map[string]string{
		"depends_on_id": dependsOnID,
		"type":          depType,
	}

	resp, err := c.post(path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// RemoveDependency removes a dependency.
func (c *Client) RemoveDependency(wsID, issueID, dependsOnID string) error {
	path := fmt.Sprintf("/api/v1/workspaces/%s/issues/%s/deps/%s", wsID, issueID, dependsOnID)

	resp, err := c.delete(path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// Statistics

// GetStatistics returns workspace statistics.
func (c *Client) GetStatistics(wsID string) (*types.Statistics, error) {
	path := fmt.Sprintf("/api/v1/workspaces/%s/stats", wsID)

	resp, err := c.get(path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var stats types.Statistics
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &stats, nil
}

// HTTP helpers

func (c *Client) get(path string) (*http.Response, error) {
	req, err := http.NewRequest("GET", c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Actor", c.actor)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if err := c.checkError(resp); err != nil {
		resp.Body.Close()
		return nil, err
	}

	return resp, nil
}

func (c *Client) post(path string, body interface{}) (*http.Response, error) {
	return c.doJSON("POST", path, body)
}

func (c *Client) put(path string, body interface{}) (*http.Response, error) {
	return c.doJSON("PUT", path, body)
}

func (c *Client) delete(path string) (*http.Response, error) {
	req, err := http.NewRequest("DELETE", c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Actor", c.actor)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if err := c.checkError(resp); err != nil {
		resp.Body.Close()
		return nil, err
	}

	return resp, nil
}

func (c *Client) doJSON(method, path string, body interface{}) (*http.Response, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal body: %w", err)
	}

	req, err := http.NewRequest(method, c.baseURL+path, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Actor", c.actor)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if err := c.checkError(resp); err != nil {
		resp.Body.Close()
		return nil, err
	}

	return resp, nil
}

func (c *Client) checkError(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	body, _ := io.ReadAll(resp.Body)

	var errResp struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(body, &errResp) == nil && errResp.Error != "" {
		return fmt.Errorf("%s", errResp.Error)
	}

	return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
}
