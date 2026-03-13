// Package client provides an HTTP API client for the arc server.
// It wraps REST endpoints for projects, issues, plans, and workspace paths.
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
// It provides methods for all CRUD operations on projects, issues,
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
// The client defaults to a 30-second timeout and "cli" as the actor identity.
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

// Health checks the server health by sending a GET /health request.
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

// Project methods provide CRUD operations for arc projects.

// ListProjects returns all projects.
func (c *Client) ListProjects() ([]*types.Project, error) {
	resp, err := c.get("/api/v1/projects")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var projects []*types.Project
	if err := json.NewDecoder(resp.Body).Decode(&projects); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return projects, nil
}

// CreateProject creates a new project.
func (c *Client) CreateProject(name, prefix, description string) (*types.Project, error) {
	body := map[string]string{
		"name":        name,
		"prefix":      prefix,
		"description": description,
	}

	resp, err := c.post("/api/v1/projects", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var proj types.Project
	if err := json.NewDecoder(resp.Body).Decode(&proj); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &proj, nil
}

// GetProject retrieves a project by ID.
func (c *Client) GetProject(id string) (*types.Project, error) {
	resp, err := c.get("/api/v1/projects/" + id)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var proj types.Project
	if err := json.NewDecoder(resp.Body).Decode(&proj); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &proj, nil
}

// UpdateProject updates a project.
func (c *Client) UpdateProject(id string, updates map[string]any) (*types.Project, error) {
	resp, err := c.put("/api/v1/projects/"+id, updates)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var proj types.Project
	if err := json.NewDecoder(resp.Body).Decode(&proj); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &proj, nil
}

// DeleteProject deletes a project.
func (c *Client) DeleteProject(id string) error {
	resp, err := c.delete("/api/v1/projects/" + id)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// MergeProjects merges one or more source projects into a target project.
func (c *Client) MergeProjects(targetID string, sourceIDs []string) (*types.MergeResult, error) {
	body := map[string]any{
		"target_id":  targetID,
		"source_ids": sourceIDs,
	}
	resp, err := c.post("/api/v1/projects/merge", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result types.MergeResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

// GetProjectStats returns project statistics.
func (c *Client) GetProjectStats(projID string) (*types.Statistics, error) {
	path := fmt.Sprintf("/api/v1/projects/%s/stats", projID)

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

// Issue methods provide CRUD operations for issues within a project.

// ListIssues returns issues for a project.
func (c *Client) ListIssues(projID string, opts ListIssuesOptions) ([]*types.Issue, error) {
	path := fmt.Sprintf("/api/v1/projects/%s/issues", projID)

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
	if opts.Parent != "" {
		query.Set("parent_id", opts.Parent)
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
	Parent   string // Filter by parent issue ID
}

// CreateIssue creates a new issue.
func (c *Client) CreateIssue(projID string, req CreateIssueRequest) (*types.Issue, error) {
	path := fmt.Sprintf("/api/v1/projects/%s/issues", projID)

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
func (c *Client) GetIssue(projID, id string) (*types.Issue, error) {
	path := fmt.Sprintf("/api/v1/projects/%s/issues/%s", projID, id)

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
func (c *Client) GetIssueDetails(projID, id string) (*types.IssueDetails, error) {
	path := fmt.Sprintf("/api/v1/projects/%s/issues/%s?details=true", projID, id)

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
func (c *Client) UpdateIssue(projID, id string, updates map[string]any) (*types.Issue, error) {
	path := fmt.Sprintf("/api/v1/projects/%s/issues/%s", projID, id)

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

// CloseIssue closes an issue. When cascade is true, open children are closed
// recursively. When cascade is false and the issue has open children, a
// *types.OpenChildrenError is returned so the caller can prompt the user.
func (c *Client) CloseIssue(projID, id, reason string, cascade bool) (*types.Issue, error) {
	path := fmt.Sprintf("/api/v1/projects/%s/issues/%s/close", projID, id)

	body := map[string]any{"reason": reason, "cascade": cascade}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal body: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL+path, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Actor", c.actor)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Handle 409 Conflict with open_children code
	if resp.StatusCode == http.StatusConflict {
		respBody, _ := io.ReadAll(resp.Body)
		var conflictResp struct {
			Error        string        `json:"error"`
			Code         string        `json:"code"`
			OpenChildren []types.Issue `json:"open_children"`
		}
		if json.Unmarshal(respBody, &conflictResp) == nil && conflictResp.Code == "open_children" {
			return nil, &types.OpenChildrenError{
				IssueID:  id,
				Children: conflictResp.OpenChildren,
			}
		}
		return nil, fmt.Errorf("%s", string(respBody))
	}

	if err := c.checkError(resp); err != nil {
		return nil, err
	}

	var issue types.Issue
	if err := json.NewDecoder(resp.Body).Decode(&issue); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &issue, nil
}

// DeleteIssue deletes an issue.
func (c *Client) DeleteIssue(projID, id string) error {
	path := fmt.Sprintf("/api/v1/projects/%s/issues/%s", projID, id)

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
func (c *Client) GetReadyWork(projID string, limit int, sortPolicy string) ([]*types.Issue, error) {
	path := fmt.Sprintf("/api/v1/projects/%s/ready", projID)

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
func (c *Client) GetBlockedIssues(projID string, limit int) ([]*types.BlockedIssue, error) {
	path := fmt.Sprintf("/api/v1/projects/%s/blocked", projID)
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
func (c *Client) AddDependency(projID, issueID, dependsOnID, depType string) error {
	path := fmt.Sprintf("/api/v1/projects/%s/issues/%s/deps", projID, issueID)

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
func (c *Client) RemoveDependency(projID, issueID, dependsOnID string) error {
	path := fmt.Sprintf("/api/v1/projects/%s/issues/%s/deps/%s", projID, issueID, dependsOnID)

	resp, err := c.delete(path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// --- Plan methods ---

// SetInlinePlan sets or updates an inline plan on an issue.
func (c *Client) SetInlinePlan(projID, issueID, text string) (*types.Comment, error) {
	path := fmt.Sprintf("/api/v1/projects/%s/issues/%s/plan", projID, issueID)

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
func (c *Client) GetPlanContext(projID, issueID string) (*types.PlanContext, error) {
	path := fmt.Sprintf("/api/v1/projects/%s/issues/%s/plan", projID, issueID)

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
func (c *Client) GetPlanHistory(projID, issueID string) ([]*types.Comment, error) {
	path := fmt.Sprintf("/api/v1/projects/%s/issues/%s/plan/history", projID, issueID)

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

// ListPlans returns all shared plans in a project.
func (c *Client) ListPlans(projID string) ([]*types.Plan, error) {
	path := fmt.Sprintf("/api/v1/projects/%s/plans", projID)

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
func (c *Client) CreatePlan(projID, title, content string) (*types.Plan, error) {
	path := fmt.Sprintf("/api/v1/projects/%s/plans", projID)

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
func (c *Client) GetPlan(projID, planID string) (*types.Plan, error) {
	path := fmt.Sprintf("/api/v1/projects/%s/plans/%s", projID, planID)

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
func (c *Client) UpdatePlan(projID, planID, title, content string) (*types.Plan, error) {
	path := fmt.Sprintf("/api/v1/projects/%s/plans/%s", projID, planID)

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
func (c *Client) DeletePlan(projID, planID string) error {
	path := fmt.Sprintf("/api/v1/projects/%s/plans/%s", projID, planID)

	resp, err := c.delete(path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// LinkIssuesToPlan links one or more issues to a plan.
func (c *Client) LinkIssuesToPlan(projID, planID string, issueIDs []string) error {
	path := fmt.Sprintf("/api/v1/projects/%s/plans/%s/link", projID, planID)

	body := map[string][]string{"issue_ids": issueIDs}
	resp, err := c.post(path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// UnlinkIssueFromPlan removes a link between an issue and a plan.
func (c *Client) UnlinkIssueFromPlan(projID, planID, issueID string) error {
	path := fmt.Sprintf("/api/v1/projects/%s/plans/%s/link/%s", projID, planID, issueID)

	resp, err := c.delete(path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// Workspace types and methods manage directory paths associated with projects.

// CreateWorkspaceRequest is the request for registering a workspace (directory path).
type CreateWorkspaceRequest struct {
	Path      string `json:"path"`
	Label     string `json:"label,omitempty"`
	Hostname  string `json:"hostname,omitempty"`
	GitRemote string `json:"git_remote,omitempty"`
	PathType  string `json:"path_type,omitempty"`
}

// ListWorkspaces returns all workspaces (directory paths) for a project.
func (c *Client) ListWorkspaces(projID string) ([]*types.Workspace, error) {
	path := fmt.Sprintf("/api/v1/projects/%s/workspaces", projID)

	resp, err := c.get(path)
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

// CreateWorkspace registers a new workspace (directory path) for a project.
func (c *Client) CreateWorkspace(projID string, req CreateWorkspaceRequest) (*types.Workspace, error) {
	path := fmt.Sprintf("/api/v1/projects/%s/workspaces", projID)

	resp, err := c.post(path, req)
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

// UpdateWorkspace updates a workspace's metadata.
func (c *Client) UpdateWorkspace(projID, wsID string, updates map[string]string) (*types.Workspace, error) {
	path := fmt.Sprintf("/api/v1/projects/%s/workspaces/%s", projID, wsID)

	resp, err := c.patch(path, updates)
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

// DeleteWorkspace removes a workspace (directory path) from a project.
func (c *Client) DeleteWorkspace(projID, wsID string) error {
	path := fmt.Sprintf("/api/v1/projects/%s/workspaces/%s", projID, wsID)

	resp, err := c.delete(path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// ResolveProjectByPath finds the project associated with a filesystem path.
func (c *Client) ResolveProjectByPath(fsPath string) (*types.ProjectResolution, error) {
	path := "/api/v1/projects/resolve?path=" + url.QueryEscape(fsPath)

	resp, err := c.get(path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result types.ProjectResolution
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

// AI Session methods provide CRUD operations for AI coding sessions.

// AISessionResponse extends AISession with its agents for detail views.
type AISessionResponse struct {
	types.AISession
	Agents []*types.AIAgent `json:"agents"`
}

// CreateAISession creates a new AI session.
func (c *Client) CreateAISession(session *types.AISession) (*types.AISession, error) {
	resp, err := c.post("/api/v1/ai/sessions", session)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result types.AISession
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

// GetAISession retrieves an AI session by ID. The response includes the Agents field.
func (c *Client) GetAISession(id string) (*AISessionResponse, error) {
	resp, err := c.get("/api/v1/ai/sessions/" + id)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result AISessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

// ListAISessions returns a paginated list of AI sessions.
func (c *Client) ListAISessions(limit, offset int) ([]*types.AISession, error) {
	path := "/api/v1/ai/sessions"

	query := url.Values{}
	if limit > 0 {
		query.Set("limit", strconv.Itoa(limit))
	}
	if offset > 0 {
		query.Set("offset", strconv.Itoa(offset))
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
		Data []*types.AISession `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return result.Data, nil
}

// DeleteAISession deletes an AI session by ID.
func (c *Client) DeleteAISession(id string) error {
	resp, err := c.delete("/api/v1/ai/sessions/" + id)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// AI Agent methods manage sub-agents within AI sessions.

// CreateAIAgent creates a new AI agent for a session.
func (c *Client) CreateAIAgent(sessionID string, agent *types.AIAgent) (*types.AIAgent, error) {
	path := fmt.Sprintf("/api/v1/ai/sessions/%s/agents", sessionID)

	resp, err := c.post(path, agent)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result types.AIAgent
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

// ListAIAgents returns all agents for a session.
func (c *Client) ListAIAgents(sessionID string) ([]*types.AIAgent, error) {
	path := fmt.Sprintf("/api/v1/ai/sessions/%s/agents", sessionID)

	resp, err := c.get(path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var agents []*types.AIAgent
	if err := json.NewDecoder(resp.Body).Decode(&agents); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return agents, nil
}

// GetAIAgent retrieves a single agent by ID within a session.
func (c *Client) GetAIAgent(sessionID, agentID string) (*types.AIAgent, error) {
	path := fmt.Sprintf("/api/v1/ai/sessions/%s/agents/%s", sessionID, agentID)

	resp, err := c.get(path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var agent types.AIAgent
	if err := json.NewDecoder(resp.Body).Decode(&agent); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &agent, nil
}

// GetAgentTranscript retrieves the transcript for an agent as a slice of
// JSON objects. The transcript is read from the agent's JSONL file on the server.
func (c *Client) GetAgentTranscript(sessionID, agentID string) ([]map[string]any, error) {
	path := fmt.Sprintf("/api/v1/ai/sessions/%s/agents/%s/transcript", sessionID, agentID)

	resp, err := c.get(path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var entries []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return entries, nil
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

	resp, err := c.httpClient.Do(req)
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

// patch performs an HTTP PATCH request with a JSON body.
func (c *Client) patch(path string, body any) (*http.Response, error) {
	return c.doJSON("PATCH", path, body)
}

// delete performs an HTTP DELETE request to the given path.
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

	resp, err := c.httpClient.Do(req)
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
// It reads the response body and attempts to extract a structured error message.
// Falls back to including the raw body text when JSON parsing fails.
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
