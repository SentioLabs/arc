// Package client provides an API client for the arc server.
package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/sentiolabs/arc/internal/types"
)

// defaultHTTPTimeoutSeconds is the default timeout for HTTP requests.
const defaultHTTPTimeoutSeconds = 30

// Client is the HTTP API client for the arc issue tracking server.
// It provides methods for all CRUD operations on workspaces, issues,
// dependencies, labels, plans, and comments.
type Client struct {
	// baseURL is the arc server URL (e.g., "http://localhost:8080").
	baseURL string
	// httpClient is the underlying HTTP client with timeout configuration.
	httpClient *http.Client
	// actor identifies the user making requests via the X-Actor header.
	actor string
}

// New creates a new API client configured to connect to the given base URL.
func New(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: defaultHTTPTimeoutSeconds * time.Second,
		},
		actor: "cli",
	}
}

// SetActor sets the actor identity sent via the X-Actor header on all requests.
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

// Workspace methods provide CRUD operations for arc workspaces.

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

// Issue methods provide CRUD operations for issues within a workspace.

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
		query.Set("limit", strconv.Itoa(opts.Limit))
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
// All fields are optional; zero values are omitted from the query.
type ListIssuesOptions struct {
	Status   string // Filter by status (e.g., "open", "closed")
	Type     string // Filter by issue type (e.g., "bug", "feature")
	Assignee string // Filter by assignee name
	Query    string // Full-text search in title/description
	Limit    int    // Maximum number of results
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
func (c *Client) UpdateIssue(wsID, id string, updates map[string]any) (*types.Issue, error) {
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

// Ready work methods return issues based on their dependency resolution status.

// GetReadyWork returns issues ready to work on.
// sortPolicy can be: "hybrid" (default), "priority", or "oldest".
func (c *Client) GetReadyWork(wsID string, limit int, sortPolicy string) ([]*types.Issue, error) {
	path := fmt.Sprintf("/api/v1/workspaces/%s/ready", wsID)

	query := url.Values{}
	if limit > 0 {
		query.Set("limit", strconv.Itoa(limit))
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

// Dependency methods manage relationships between issues (blocks, parent-child, related).

// AddDependency adds a dependency between two issues.
// depType should be one of: "blocks", "parent-child", "related", "discovered-from".
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

// RemoveDependency removes a dependency between two issues.
func (c *Client) RemoveDependency(wsID, issueID, dependsOnID string) error {
	path := fmt.Sprintf("/api/v1/workspaces/%s/issues/%s/deps/%s", wsID, issueID, dependsOnID)

	resp, err := c.delete(path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// Statistics methods provide aggregated metrics for a workspace.

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

// --- Plan methods ---

// SetInlinePlan sets or updates an inline plan on an issue.
func (c *Client) SetInlinePlan(wsID, issueID, text string) (*types.Comment, error) {
	path := fmt.Sprintf("/api/v1/workspaces/%s/issues/%s/plan", wsID, issueID)

	body := map[string]string{"text": text}
	resp, err := c.post(path, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var comment types.Comment
	if err := json.NewDecoder(resp.Body).Decode(&comment); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &comment, nil
}

// GetPlanContext returns the plan context for an issue.
func (c *Client) GetPlanContext(wsID, issueID string) (*types.PlanContext, error) {
	path := fmt.Sprintf("/api/v1/workspaces/%s/issues/%s/plan", wsID, issueID)

	resp, err := c.get(path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var pc types.PlanContext
	if err := json.NewDecoder(resp.Body).Decode(&pc); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &pc, nil
}

// GetPlanHistory returns the plan version history for an issue.
func (c *Client) GetPlanHistory(wsID, issueID string) ([]*types.Comment, error) {
	path := fmt.Sprintf("/api/v1/workspaces/%s/issues/%s/plan/history", wsID, issueID)

	resp, err := c.get(path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var comments []*types.Comment
	if err := json.NewDecoder(resp.Body).Decode(&comments); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return comments, nil
}

// ListPlans returns all shared plans in a workspace.
func (c *Client) ListPlans(wsID string) ([]*types.Plan, error) {
	path := fmt.Sprintf("/api/v1/workspaces/%s/plans", wsID)

	resp, err := c.get(path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var plans []*types.Plan
	if err := json.NewDecoder(resp.Body).Decode(&plans); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return plans, nil
}

// CreatePlan creates a new shared plan.
func (c *Client) CreatePlan(wsID, title, content string) (*types.Plan, error) {
	path := fmt.Sprintf("/api/v1/workspaces/%s/plans", wsID)

	body := map[string]string{"title": title, "content": content}
	resp, err := c.post(path, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var plan types.Plan
	if err := json.NewDecoder(resp.Body).Decode(&plan); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &plan, nil
}

// GetPlan retrieves a plan by ID.
func (c *Client) GetPlan(wsID, planID string) (*types.Plan, error) {
	path := fmt.Sprintf("/api/v1/workspaces/%s/plans/%s", wsID, planID)

	resp, err := c.get(path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var plan types.Plan
	if err := json.NewDecoder(resp.Body).Decode(&plan); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &plan, nil
}

// UpdatePlan updates a shared plan.
func (c *Client) UpdatePlan(wsID, planID, title, content string) (*types.Plan, error) {
	path := fmt.Sprintf("/api/v1/workspaces/%s/plans/%s", wsID, planID)

	body := map[string]string{"title": title, "content": content}
	resp, err := c.put(path, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var plan types.Plan
	if err := json.NewDecoder(resp.Body).Decode(&plan); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &plan, nil
}

// DeletePlan deletes a shared plan.
func (c *Client) DeletePlan(wsID, planID string) error {
	path := fmt.Sprintf("/api/v1/workspaces/%s/plans/%s", wsID, planID)

	resp, err := c.delete(path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// LinkIssuesToPlan links one or more issues to a plan.
func (c *Client) LinkIssuesToPlan(wsID, planID string, issueIDs []string) error {
	path := fmt.Sprintf("/api/v1/workspaces/%s/plans/%s/link", wsID, planID)

	body := map[string][]string{"issue_ids": issueIDs}
	resp, err := c.post(path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// UnlinkIssueFromPlan removes a link between an issue and a plan.
func (c *Client) UnlinkIssueFromPlan(wsID, planID, issueID string) error {
	path := fmt.Sprintf("/api/v1/workspaces/%s/plans/%s/link/%s", wsID, planID, issueID)

	resp, err := c.delete(path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// HTTP helpers - low-level methods for making requests to the arc server.
// All methods set the X-Actor header and check for error responses.

// get performs an HTTP GET request to the given path.
func (c *Client) get(path string) (*http.Response, error) {
	req, err := http.NewRequest("GET", c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Actor", c.actor)

	resp, err := c.httpClient.Do(req) //nolint:gosec // G704: URL is built from user-configured arc server base URL
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if err := c.checkError(resp); err != nil {
		_ = resp.Body.Close()
		return nil, err
	}

	return resp, nil
}

// post performs an HTTP POST request with a JSON body.
func (c *Client) post(path string, body any) (*http.Response, error) {
	return c.doJSON("POST", path, body)
}

// put performs an HTTP PUT request with a JSON body.
func (c *Client) put(path string, body any) (*http.Response, error) {
	return c.doJSON("PUT", path, body)
}

// delete performs an HTTP DELETE request to the given path.
func (c *Client) delete(path string) (*http.Response, error) {
	req, err := http.NewRequest("DELETE", c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Actor", c.actor)

	resp, err := c.httpClient.Do(req) //nolint:gosec // G704: URL is built from user-configured arc server base URL
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if err := c.checkError(resp); err != nil {
		_ = resp.Body.Close()
		return nil, err
	}

	return resp, nil
}

// doJSON performs an HTTP request with the given method and a JSON-encoded body.
func (c *Client) doJSON(method, path string, body any) (*http.Response, error) {
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

	resp, err := c.httpClient.Do(req) //nolint:gosec // G704: URL is built from user-configured arc server base URL
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if err := c.checkError(resp); err != nil {
		_ = resp.Body.Close()
		return nil, err
	}

	return resp, nil
}

// checkError inspects the HTTP response status and returns an error for non-2xx codes.
// It attempts to parse the error message from the JSON response body.
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
