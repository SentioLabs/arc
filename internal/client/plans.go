package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/google/uuid"
	"github.com/sentiolabs/arc/internal/storage"
	"github.com/sentiolabs/arc/internal/types"
)

// APIError preserves the server's status, code, and complete conflict details.
type APIError struct {
	StatusCode int             `json:"-"`
	Code       string          `json:"code"`
	Message    string          `json:"error"`
	Body       json.RawMessage `json:"-"`
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("%s (HTTP %d): %s", e.Message, e.StatusCode, e.Body)
	}
	return fmt.Sprintf("request failed with status %d: %s", e.StatusCode, e.Body)
}

// PlanRequestUncertainError retains the only safe replay identity after a
// response loss, including when a later attempt returns a structured HTTP error.
type PlanRequestUncertainError struct {
	IdempotencyKey string
	Err            error
}

func (e *PlanRequestUncertainError) Error() string {
	return fmt.Sprintf(
		"request outcome uncertain; retry with --idempotency-key %s and identical input: %v",
		e.IdempotencyKey,
		e.Err,
	)
}

// Unwrap preserves access to the underlying API status, code, and details.
func (e *PlanRequestUncertainError) Unwrap() error { return e.Err }

// planRequestOptions distinguishes durable keyed mutations from single attempts.
type planRequestOptions struct {
	key   string
	retry bool
}

// planRequest retries only operations whose server contract durably deduplicates
// a key. Encoding and key generation happen once, including for lost responses.
func planRequest[T any](
	c *Client,
	method, path string,
	body any,
	options planRequestOptions,
) (*T, error) {
	return planRequestContext[T](context.Background(), c, method, path, body, options)
}

// planRequestContext binds the request and body reads to the caller's lifetime.
// It checks late responses as well as transport cancellation before success.
//
//nolint:revive // Context plus the shared transport's method/path/payload/options form one request.
func planRequestContext[T any](
	ctx context.Context,
	c *Client,
	method, path string,
	body any,
	options planRequestOptions,
) (*T, error) {
	key, retry := options.key, options.retry
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal body: %w", err)
	}
	if retry && key == "" {
		key = uuid.NewString()
	}
	attempts := 1
	if retry {
		attempts = 2
	}
	uncertain := false
	for range attempts {
		req, reqErr := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(data))
		if reqErr != nil {
			return nil, reqErr
		}
		req.Header.Set("Content-Type", "application/json")
		c.setRequestIdentity(req)
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		resp, sendErr := c.httpClient.Do(req)
		if sendErr != nil {
			err = sendErr
			uncertain = true
			continue
		}
		if statusErr := c.checkError(resp); statusErr != nil {
			_ = resp.Body.Close()
			if retry && (uncertain || resp.StatusCode >= http.StatusInternalServerError) {
				return nil, &PlanRequestUncertainError{IdempotencyKey: key, Err: statusErr}
			}
			return nil, statusErr
		}
		var result T
		err = json.NewDecoder(resp.Body).Decode(&result)
		_ = resp.Body.Close()
		if err == nil {
			err = ctx.Err()
		}
		if err == nil {
			return &result, nil
		}
		uncertain = true
	}
	if retry {
		return nil, &PlanRequestUncertainError{IdempotencyKey: key, Err: err}
	}

	return nil, fmt.Errorf("request failed: %w", err)
}

// planPath keeps every durable operation inside an explicit project boundary.
// IDs are escaped as path components, never interpreted as filesystem paths.
func planPath(projectID, planID string) string {
	path := "/api/v1/projects/" + url.PathEscape(projectID) + "/plans"
	if planID != "" {
		path += "/" + url.PathEscape(planID)
	}
	return path
}

// revisionPath names an immutable retained revision.
// No client operation substitutes a head for this caller-supplied number.
func revisionPath(projectID, planID string, revision int64) string {
	return fmt.Sprintf("%s/revisions/%d", planPath(projectID, planID), revision)
}

// planPagePath passes bounded pagination to the server.
// The service validates limits and offsets before reading retained history.
func planPagePath(path string, limit, offset int) string {
	return fmt.Sprintf("%s?limit=%d&offset=%d", path, limit, offset)
}

// issuePlanPath scopes governance and evidence to the owning project.
// The server additionally checks that the issue belongs to this project.
func issuePlanPath(projectID, issueID, suffix string) string {
	return "/api/v1/projects/" + url.PathEscape(
		projectID,
	) + "/issues/" + url.PathEscape(
		issueID,
	) + "/" + suffix
}

// CreatePlan uploads content from a client, with no server filesystem path.
func (c *Client) CreatePlan(
	projectID, key string,
	req storage.PlanUpload,
) (*storage.PlanWriteResult, error) {
	return planRequest[storage.PlanWriteResult](
		c,
		http.MethodPost,
		planPath(projectID, ""),
		req,
		planRequestOptions{key: key, retry: true},
	)
}

// GetPlan reads mutable metadata separately from immutable content.
// Its head is a display convenience, not an implicit editing precondition.
func (c *Client) GetPlan(projectID, planID string) (*types.Plan, error) {
	return c.GetPlanContext(context.Background(), projectID, planID)
}

// GetPlanContext cancels HTTP and body reads with the supplied context.
func (c *Client) GetPlanContext(ctx context.Context, projectID, planID string) (*types.Plan, error) {
	return planRequestContext[types.Plan](ctx, c,
		http.MethodGet,
		planPath(projectID, planID),
		nil,
		planRequestOptions{},
	)
}

// ListPlans reads one active or archived page without deleting history.
// Archival preserves content and references under the same plan identity.
func (c *Client) ListPlans(
	projectID string,
	archived bool,
	limit, offset int,
) ([]*types.Plan, error) {
	result, err := planRequest[[]*types.Plan](
		c,
		http.MethodGet,
		planPagePath(planPath(projectID, ""), limit, offset)+fmt.Sprintf("&archived=%t", archived),
		nil,
		planRequestOptions{},
	)
	if err != nil {
		return nil, err
	}
	return *result, nil
}

// ReadPlanRevision reads verified content for the exact supplied revision.
// Server integrity failures retain their status and diagnostic response.
func (c *Client) ReadPlanRevision(
	projectID, planID string,
	revision int64,
) (*types.PlanRevisionWithContent, error) {
	return c.ReadPlanRevisionContext(context.Background(), projectID, planID, revision)
}

// ReadPlanRevisionContext cancels HTTP and body reads with the supplied context.
func (c *Client) ReadPlanRevisionContext(
	ctx context.Context,
	projectID, planID string,
	revision int64,
) (*types.PlanRevisionWithContent, error) {
	return planRequestContext[types.PlanRevisionWithContent](ctx, c,
		http.MethodGet,
		revisionPath(projectID, planID, revision),
		nil,
		planRequestOptions{},
	)
}

// SavePlanRevision creates an immutable revision against the supplied editing base.
// Transport retries use the same key and marshaled request bytes.
func (c *Client) SavePlanRevision(
	projectID, planID, key string,
	req storage.PlanSave,
) (*storage.PlanWriteResult, error) {
	return planRequest[storage.PlanWriteResult](
		c,
		http.MethodPost,
		planPath(projectID, planID)+"/revisions",
		req,
		planRequestOptions{key: key, retry: true},
	)
}

// ListPlanRevisions reads historical review state in bounded pages.
// A later content save never rewrites decisions on these earlier revisions.
func (c *Client) ListPlanRevisions(
	projectID, planID string,
	limit, offset int,
) ([]*types.PlanRevision, error) {
	result, err := planRequest[[]*types.PlanRevision](
		c,
		http.MethodGet,
		planPagePath(planPath(projectID, planID)+"/revisions", limit, offset),
		nil,
		planRequestOptions{},
	)
	if err != nil {
		return nil, err
	}
	return *result, nil
}

// UpdatePlan changes title or lifecycle using the captured metadata version.
// It is not retried automatically because the server does not deduplicate this route.
func (c *Client) UpdatePlan(
	projectID, planID string,
	req storage.PlanMetadataUpdate,
) (*types.Plan, error) {
	return planRequest[types.Plan](
		c,
		http.MethodPatch,
		planPath(projectID, planID),
		req,
		planRequestOptions{},
	)
}

// DecidePlanRevision forwards all captured head, review, and feedback versions.
// An uncertain response is returned without replaying an unkeyed decision.
func (c *Client) DecidePlanRevision(
	projectID, planID string,
	revision int64,
	req types.PlanReviewRequest,
) (*types.PlanRevision, error) {
	return planRequest[types.PlanRevision](
		c,
		http.MethodPost,
		revisionPath(projectID, planID, revision)+"/decisions",
		req,
		planRequestOptions{},
	)
}

// CreatePlanComment binds feedback to its original revision and anchor.
// Creation is attempted once because comments have no durable request key.
func (c *Client) CreatePlanComment(
	projectID, planID string,
	revision int64,
	req storage.PlanCommentCreate,
) (*types.PlanComment, error) {
	return planRequest[types.PlanComment](
		c,
		http.MethodPost,
		revisionPath(projectID, planID, revision)+"/comments",
		req,
		planRequestOptions{},
	)
}

// UpdatePlanComment applies a version-checked edit or reopening.
// Its original revision remains part of the ownership boundary.
func (c *Client) UpdatePlanComment(
	projectID, planID string,
	revision int64,
	commentID string,
	req storage.PlanCommentUpdate,
) (*types.PlanComment, error) {
	return planRequest[types.PlanComment](
		c,
		http.MethodPatch,
		revisionPath(projectID, planID, revision)+"/comments/"+url.PathEscape(commentID),
		req,
		planRequestOptions{},
	)
}

// DeletePlanComment creates a version-checked tombstone.
// The retained audit discussion cannot be erased through the client.
func (c *Client) DeletePlanComment(
	projectID, planID string,
	revision int64,
	commentID string,
	version int64,
) (*types.PlanComment, error) {
	return planRequest[types.PlanComment](
		c,
		http.MethodDelete,
		revisionPath(projectID, planID, revision)+"/comments/"+url.PathEscape(commentID),
		storage.PlanCommentUpdate{ExpectedVersion: version},
		planRequestOptions{},
	)
}

// ListPlanComments optionally includes unresolved earlier feedback.
// The original revision and anchor remain attached to every returned comment.
//
//nolint:revive // The project/plan/revision ownership, prior filter and pagination mirror the service contract.
func (c *Client) ListPlanComments(
	projectID, planID string,
	revision int64,
	prior bool,
	limit, offset int,
) ([]*types.PlanComment, error) {
	return c.ListPlanCommentsContext(context.Background(), projectID, planID, revision, prior, limit, offset)
}

// ListPlanCommentsContext cancels HTTP and body reads with the supplied context.
//
//nolint:revive // Exact ownership, prior filter, and pagination remain explicit alongside cancellation.
func (c *Client) ListPlanCommentsContext(
	ctx context.Context,
	projectID, planID string,
	revision int64,
	prior bool,
	limit, offset int,
) ([]*types.PlanComment, error) {
	result, err := planRequestContext[[]*types.PlanComment](ctx, c,
		http.MethodGet,
		planPagePath(
			revisionPath(projectID, planID, revision)+"/comments",
			limit,
			offset,
		)+fmt.Sprintf(
			"&include_prior=%t",
			prior,
		),
		nil,
		planRequestOptions{},
	)
	if err != nil {
		return nil, err
	}
	return *result, nil
}

// AddPlanDisposition records an addressed or deferred reason append-only.
// The request binds the exact comment version and captured feedback counter.
func (c *Client) AddPlanDisposition(
	projectID, planID string,
	revision int64,
	req storage.PlanDispositionRequest,
) (*types.PlanFeedbackDisposition, error) {
	return planRequest[types.PlanFeedbackDisposition](
		c,
		http.MethodPost,
		revisionPath(projectID, planID, revision)+"/dispositions",
		req,
		planRequestOptions{},
	)
}

// ListPlanDispositions reads recorded reasons through a target revision.
// Clients can inspect carry-forward addressed items and revision-specific deferrals.
func (c *Client) ListPlanDispositions(
	projectID, planID string,
	revision int64,
	limit, offset int,
) ([]*types.PlanFeedbackDisposition, error) {
	result, err := planRequest[[]*types.PlanFeedbackDisposition](
		c,
		http.MethodGet,
		planPagePath(revisionPath(projectID, planID, revision)+"/dispositions", limit, offset),
		nil,
		planRequestOptions{},
	)
	if err != nil {
		return nil, err
	}
	return *result, nil
}

// AdoptPlan atomically applies a reconciliation or validates it without writes.
// Only apply uses a durable key; dry-runs do not reserve a request identity.
func (c *Client) AdoptPlan(
	projectID, issueID, key string,
	req types.PlanAdoptionRequest,
) (*storage.PlanAdoptionResult, error) {
	return planRequest[storage.PlanAdoptionResult](
		c,
		http.MethodPost,
		issuePlanPath(projectID, issueID, "plan-adoption"),
		req,
		planRequestOptions{key: key, retry: !req.DryRun},
	)
}

// ResolveGoverningPlan returns the complete pinned source chain.
// A successful JSON null explicitly represents valid unlinked work.
func (c *Client) ResolveGoverningPlan(projectID, issueID string) (*types.GoverningPlan, error) {
	result, err := planRequest[*types.GoverningPlan](
		c,
		http.MethodGet,
		issuePlanPath(projectID, issueID, "governing-plan"),
		nil,
		planRequestOptions{},
	)
	if err != nil {
		return nil, err
	}
	return *result, nil
}

// RecordExecutionEvidence sends the original complete work expectation.
// It never reads current governance to replace a stale worker capture.
func (c *Client) RecordExecutionEvidence(
	projectID, issueID string,
	req types.ExecutionEvidenceRequest,
) (*storage.ExecutionEvidence, error) {
	return planRequest[storage.ExecutionEvidence](
		c,
		http.MethodPost,
		issuePlanPath(projectID, issueID, "execution-evidence"),
		req,
		planRequestOptions{},
	)
}

// ListExecutionEvidence reads immutable phase provenance in bounded pages.
// Later adoption does not rewrite the captured historical source chain.
func (c *Client) ListExecutionEvidence(
	projectID, issueID string,
	limit, offset int,
) ([]storage.ExecutionEvidence, error) {
	result, err := planRequest[[]storage.ExecutionEvidence](
		c,
		http.MethodGet,
		planPagePath(issuePlanPath(projectID, issueID, "execution-evidence"), limit, offset),
		nil,
		planRequestOptions{},
	)
	if err != nil {
		return nil, err
	}
	return *result, nil
}

// UpdateIssueWithContext decodes the full, transactional post-update response.
func (c *Client) UpdateIssueWithContext(
	issueID string,
	updates map[string]any,
) (*types.IssueDetails, error) {
	return planRequest[types.IssueDetails](
		c,
		http.MethodPut,
		"/api/v1/issues/"+url.PathEscape(issueID),
		updates,
		planRequestOptions{},
	)
}

// CloseIssueWithContext enforces an original work capture on explicit completion.
// It retains typed open-child errors alongside structured governance conflicts.
func (c *Client) CloseIssueWithContext(
	issueID, reason string,
	cascade bool,
	expected *types.ExpectedGovernance,
) (*types.Issue, error) {
	result, err := planRequest[types.Issue](
		c,
		http.MethodPost,
		"/api/v1/issues/"+url.PathEscape(issueID)+"/close",
		map[string]any{"reason": reason, "cascade": cascade, "expected": expected},
		planRequestOptions{},
	)
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.Code == "open_children" {
		var detail struct {
			Children []types.Issue `json:"open_children"`
		}
		if json.Unmarshal(apiErr.Body, &detail) == nil {
			return nil, &types.OpenChildrenError{IssueID: issueID, Children: detail.Children}
		}
	}
	return result, err
}
