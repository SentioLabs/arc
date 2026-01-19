package api

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/sentiolabs/arc/internal/service"
	"github.com/sentiolabs/arc/internal/types"
)

// StrictHandler implements StrictServerInterface using services.
type StrictHandler struct {
	services *service.Services
}

// NewHandler creates a new strict handler with the given services.
func NewHandler(services *service.Services) *StrictHandler {
	return &StrictHandler{services: services}
}

// getActorFromRequest extracts the actor header from the request object.
// Note: In strict mode, headers are passed via the request context.
func getActorFromRequest(ctx context.Context) string {
	// TODO: Extract from context if middleware sets it
	return "anonymous"
}

// ====================
// Workspace handlers
// ====================

func (h *StrictHandler) ListWorkspaces(ctx context.Context, request ListWorkspacesRequestObject) (ListWorkspacesResponseObject, error) {
	workspaces, err := h.services.Workspaces.List(ctx)
	if err != nil {
		return ListWorkspaces500JSONResponse{InternalErrorJSONResponse{Error: err.Error()}}, nil
	}

	result := make(ListWorkspaces200JSONResponse, len(workspaces))
	for i, ws := range workspaces {
		result[i] = workspaceToAPI(ws)
	}

	return result, nil
}

func (h *StrictHandler) CreateWorkspace(ctx context.Context, request CreateWorkspaceRequestObject) (CreateWorkspaceResponseObject, error) {
	body := request.Body
	path := ""
	desc := ""
	if body.Path != nil {
		path = *body.Path
	}
	if body.Description != nil {
		desc = *body.Description
	}

	ws, err := h.services.Workspaces.Create(ctx, body.Name, path, desc, body.Prefix)
	if err != nil {
		if strings.Contains(err.Error(), "validation") {
			return CreateWorkspace400JSONResponse{BadRequestJSONResponse{Error: err.Error()}}, nil
		}
		return CreateWorkspace500JSONResponse{InternalErrorJSONResponse{Error: err.Error()}}, nil
	}

	return CreateWorkspace201JSONResponse(workspaceToAPI(ws)), nil
}

func (h *StrictHandler) GetWorkspace(ctx context.Context, request GetWorkspaceRequestObject) (GetWorkspaceResponseObject, error) {
	ws, err := h.services.Workspaces.Get(ctx, string(request.WorkspaceID))
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return GetWorkspace404JSONResponse{NotFoundJSONResponse{Error: err.Error()}}, nil
		}
		return GetWorkspace500JSONResponse{InternalErrorJSONResponse{Error: err.Error()}}, nil
	}

	return GetWorkspace200JSONResponse(workspaceToAPI(ws)), nil
}

func (h *StrictHandler) UpdateWorkspace(ctx context.Context, request UpdateWorkspaceRequestObject) (UpdateWorkspaceResponseObject, error) {
	body := request.Body
	var name, path, desc *string
	if body.Name != nil {
		name = body.Name
	}
	if body.Path != nil {
		path = body.Path
	}
	if body.Description != nil {
		desc = body.Description
	}

	ws, err := h.services.Workspaces.Update(ctx, string(request.WorkspaceID), name, path, desc)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return UpdateWorkspace404JSONResponse{NotFoundJSONResponse{Error: err.Error()}}, nil
		}
		return UpdateWorkspace500JSONResponse{InternalErrorJSONResponse{Error: err.Error()}}, nil
	}

	return UpdateWorkspace200JSONResponse(workspaceToAPI(ws)), nil
}

func (h *StrictHandler) DeleteWorkspace(ctx context.Context, request DeleteWorkspaceRequestObject) (DeleteWorkspaceResponseObject, error) {
	if err := h.services.Workspaces.Delete(ctx, string(request.WorkspaceID)); err != nil {
		if strings.Contains(err.Error(), "not found") {
			return DeleteWorkspace404JSONResponse{NotFoundJSONResponse{Error: err.Error()}}, nil
		}
		return DeleteWorkspace500JSONResponse{InternalErrorJSONResponse{Error: err.Error()}}, nil
	}

	return DeleteWorkspace204Response{}, nil
}

func (h *StrictHandler) GetWorkspaceStats(ctx context.Context, request GetWorkspaceStatsRequestObject) (GetWorkspaceStatsResponseObject, error) {
	stats, err := h.services.Workspaces.GetStatistics(ctx, string(request.WorkspaceID))
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return GetWorkspaceStats404JSONResponse{NotFoundJSONResponse{Error: err.Error()}}, nil
		}
		return GetWorkspaceStats500JSONResponse{InternalErrorJSONResponse{Error: err.Error()}}, nil
	}

	return GetWorkspaceStats200JSONResponse(statisticsToAPI(stats)), nil
}

// ====================
// Issue handlers
// ====================

func (h *StrictHandler) ListIssues(ctx context.Context, request ListIssuesRequestObject) (ListIssuesResponseObject, error) {
	filter := types.IssueFilter{
		WorkspaceID: string(request.WorkspaceID),
	}

	if request.Params.Status != nil {
		s := types.Status(*request.Params.Status)
		filter.Status = &s
	}
	if request.Params.Type != nil {
		t := types.IssueType(*request.Params.Type)
		filter.IssueType = &t
	}
	if request.Params.Priority != nil {
		filter.Priority = request.Params.Priority
	}
	if request.Params.Assignee != nil {
		filter.Assignee = request.Params.Assignee
	}
	if request.Params.Q != nil {
		filter.Query = *request.Params.Q
	}
	if request.Params.Limit != nil {
		filter.Limit = *request.Params.Limit
	} else {
		filter.Limit = 100
	}
	if request.Params.Offset != nil {
		filter.Offset = *request.Params.Offset
	}

	issues, err := h.services.Issues.List(ctx, filter)
	if err != nil {
		return ListIssues500JSONResponse{InternalErrorJSONResponse{Error: err.Error()}}, nil
	}

	data := make([]Issue, len(issues))
	for i, issue := range issues {
		data[i] = issueToAPI(issue)
	}

	return ListIssues200JSONResponse(PaginatedIssues{
		Data:   data,
		Total:  ptrInt(len(issues)),
		Limit:  ptrInt(filter.Limit),
		Offset: ptrInt(filter.Offset),
	}), nil
}

func (h *StrictHandler) CreateIssue(ctx context.Context, request CreateIssueRequestObject) (CreateIssueResponseObject, error) {
	body := request.Body
	actor := getActorFromRequest(ctx)

	input := service.CreateIssueInput{
		WorkspaceID: string(request.WorkspaceID),
		Title:       body.Title,
	}
	if body.Description != nil {
		input.Description = *body.Description
	}
	if body.AcceptanceCriteria != nil {
		input.AcceptanceCriteria = *body.AcceptanceCriteria
	}
	if body.Notes != nil {
		input.Notes = *body.Notes
	}
	if body.Status != nil {
		s := types.Status(*body.Status)
		input.Status = &s
	}
	if body.Priority != nil {
		input.Priority = body.Priority
	}
	if body.IssueType != nil {
		t := types.IssueType(*body.IssueType)
		input.IssueType = &t
	}
	if body.Assignee != nil {
		input.Assignee = *body.Assignee
	}
	if body.ExternalRef != nil {
		input.ExternalRef = *body.ExternalRef
	}

	issue, err := h.services.Issues.Create(ctx, input, actor)
	if err != nil {
		if strings.Contains(err.Error(), "validation") {
			return CreateIssue400JSONResponse{BadRequestJSONResponse{Error: err.Error()}}, nil
		}
		return CreateIssue500JSONResponse{InternalErrorJSONResponse{Error: err.Error()}}, nil
	}

	return CreateIssue201JSONResponse(issueToAPI(issue)), nil
}

func (h *StrictHandler) GetIssue(ctx context.Context, request GetIssueRequestObject) (GetIssueResponseObject, error) {
	// Check if details requested
	if request.Params.Details != nil && *request.Params.Details {
		details, err := h.services.Issues.GetDetails(ctx, string(request.IssueID))
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				return GetIssue404JSONResponse{NotFoundJSONResponse{Error: err.Error()}}, nil
			}
			return GetIssue500JSONResponse{InternalErrorJSONResponse{Error: err.Error()}}, nil
		}
		apiIssue := issueDetailsToAPI(details)
		unionData, err := json.Marshal(apiIssue)
		if err != nil {
			return GetIssue500JSONResponse{InternalErrorJSONResponse{Error: "failed to marshal response"}}, nil
		}
		return GetIssue200JSONResponse{union: unionData}, nil
	}

	issue, err := h.services.Issues.Get(ctx, string(request.IssueID))
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return GetIssue404JSONResponse{NotFoundJSONResponse{Error: err.Error()}}, nil
		}
		return GetIssue500JSONResponse{InternalErrorJSONResponse{Error: err.Error()}}, nil
	}

	apiIssue := issueToAPI(issue)
	unionData, err := json.Marshal(apiIssue)
	if err != nil {
		return GetIssue500JSONResponse{InternalErrorJSONResponse{Error: "failed to marshal response"}}, nil
	}
	return GetIssue200JSONResponse{union: unionData}, nil
}

func (h *StrictHandler) UpdateIssue(ctx context.Context, request UpdateIssueRequestObject) (UpdateIssueResponseObject, error) {
	body := request.Body
	actor := getActorFromRequest(ctx)

	input := service.UpdateIssueInput{}
	if body.Title != nil {
		input.Title = body.Title
	}
	if body.Description != nil {
		input.Description = body.Description
	}
	if body.AcceptanceCriteria != nil {
		input.AcceptanceCriteria = body.AcceptanceCriteria
	}
	if body.Notes != nil {
		input.Notes = body.Notes
	}
	if body.Status != nil {
		s := string(*body.Status)
		input.Status = &s
	}
	if body.Priority != nil {
		input.Priority = body.Priority
	}
	if body.IssueType != nil {
		t := string(*body.IssueType)
		input.IssueType = &t
	}
	if body.Assignee != nil {
		input.Assignee = body.Assignee
	}
	if body.ExternalRef != nil {
		input.ExternalRef = body.ExternalRef
	}

	issue, err := h.services.Issues.Update(ctx, string(request.IssueID), input, actor)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return UpdateIssue404JSONResponse{NotFoundJSONResponse{Error: err.Error()}}, nil
		}
		if strings.Contains(err.Error(), "no updates") || strings.Contains(err.Error(), "invalid") {
			return UpdateIssue400JSONResponse{BadRequestJSONResponse{Error: err.Error()}}, nil
		}
		return UpdateIssue500JSONResponse{InternalErrorJSONResponse{Error: err.Error()}}, nil
	}

	return UpdateIssue200JSONResponse(issueToAPI(issue)), nil
}

func (h *StrictHandler) DeleteIssue(ctx context.Context, request DeleteIssueRequestObject) (DeleteIssueResponseObject, error) {
	if err := h.services.Issues.Delete(ctx, string(request.IssueID)); err != nil {
		if strings.Contains(err.Error(), "not found") {
			return DeleteIssue404JSONResponse{NotFoundJSONResponse{Error: err.Error()}}, nil
		}
		return DeleteIssue500JSONResponse{InternalErrorJSONResponse{Error: err.Error()}}, nil
	}

	return DeleteIssue204Response{}, nil
}

func (h *StrictHandler) CloseIssue(ctx context.Context, request CloseIssueRequestObject) (CloseIssueResponseObject, error) {
	actor := getActorFromRequest(ctx)
	reason := ""
	if request.Body != nil && request.Body.Reason != nil {
		reason = *request.Body.Reason
	}

	issue, err := h.services.Issues.Close(ctx, string(request.IssueID), reason, actor)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return CloseIssue404JSONResponse{NotFoundJSONResponse{Error: err.Error()}}, nil
		}
		return CloseIssue500JSONResponse{InternalErrorJSONResponse{Error: err.Error()}}, nil
	}

	return CloseIssue200JSONResponse(issueToAPI(issue)), nil
}

func (h *StrictHandler) ReopenIssue(ctx context.Context, request ReopenIssueRequestObject) (ReopenIssueResponseObject, error) {
	actor := getActorFromRequest(ctx)

	issue, err := h.services.Issues.Reopen(ctx, string(request.IssueID), actor)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return ReopenIssue404JSONResponse{NotFoundJSONResponse{Error: err.Error()}}, nil
		}
		return ReopenIssue500JSONResponse{InternalErrorJSONResponse{Error: err.Error()}}, nil
	}

	return ReopenIssue200JSONResponse(issueToAPI(issue)), nil
}

// ====================
// Ready/Blocked handlers
// ====================

func (h *StrictHandler) GetReadyWork(ctx context.Context, request GetReadyWorkRequestObject) (GetReadyWorkResponseObject, error) {
	filter := types.WorkFilter{
		WorkspaceID: string(request.WorkspaceID),
	}

	if request.Params.Type != nil {
		t := types.IssueType(*request.Params.Type)
		filter.IssueType = &t
	}
	if request.Params.Priority != nil {
		filter.Priority = request.Params.Priority
	}
	if request.Params.Assignee != nil {
		filter.Assignee = request.Params.Assignee
	}
	if request.Params.Unassigned != nil {
		filter.Unassigned = *request.Params.Unassigned
	}
	if request.Params.Limit != nil {
		filter.Limit = *request.Params.Limit
	} else {
		filter.Limit = 100
	}

	issues, err := h.services.Issues.GetReadyWork(ctx, filter)
	if err != nil {
		return GetReadyWork500JSONResponse{InternalErrorJSONResponse{Error: err.Error()}}, nil
	}

	result := make(GetReadyWork200JSONResponse, len(issues))
	for i, issue := range issues {
		result[i] = issueToAPI(issue)
	}

	return result, nil
}

func (h *StrictHandler) GetBlockedIssues(ctx context.Context, request GetBlockedIssuesRequestObject) (GetBlockedIssuesResponseObject, error) {
	filter := types.WorkFilter{
		WorkspaceID: string(request.WorkspaceID),
	}
	if request.Params.Limit != nil {
		filter.Limit = *request.Params.Limit
	} else {
		filter.Limit = 100
	}

	issues, err := h.services.Issues.GetBlockedIssues(ctx, filter)
	if err != nil {
		return GetBlockedIssues500JSONResponse{InternalErrorJSONResponse{Error: err.Error()}}, nil
	}

	result := make(GetBlockedIssues200JSONResponse, len(issues))
	for i, blocked := range issues {
		result[i] = blockedIssueToAPI(blocked)
	}

	return result, nil
}

// ====================
// Dependency handlers
// ====================

func (h *StrictHandler) GetDependencies(ctx context.Context, request GetDependenciesRequestObject) (GetDependenciesResponseObject, error) {
	graph, err := h.services.Dependencies.GetGraph(ctx, string(request.IssueID))
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return GetDependencies404JSONResponse{NotFoundJSONResponse{Error: err.Error()}}, nil
		}
		return GetDependencies500JSONResponse{InternalErrorJSONResponse{Error: err.Error()}}, nil
	}

	deps := make([]Dependency, len(graph.Dependencies))
	for i, d := range graph.Dependencies {
		deps[i] = dependencyToAPI(d)
	}

	dependents := make([]Dependency, len(graph.Dependents))
	for i, d := range graph.Dependents {
		dependents[i] = dependencyToAPI(d)
	}

	return GetDependencies200JSONResponse(DependencyGraph{
		Dependencies: deps,
		Dependents:   dependents,
	}), nil
}

func (h *StrictHandler) AddDependency(ctx context.Context, request AddDependencyRequestObject) (AddDependencyResponseObject, error) {
	actor := getActorFromRequest(ctx)
	body := request.Body

	dep, err := h.services.Dependencies.Add(ctx, string(request.IssueID), body.DependsOnID, types.DependencyType(body.Type), actor)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return AddDependency404JSONResponse{NotFoundJSONResponse{Error: err.Error()}}, nil
		}
		if strings.Contains(err.Error(), "invalid") || strings.Contains(err.Error(), "itself") {
			return AddDependency400JSONResponse{BadRequestJSONResponse{Error: err.Error()}}, nil
		}
		return AddDependency500JSONResponse{InternalErrorJSONResponse{Error: err.Error()}}, nil
	}

	return AddDependency201JSONResponse(dependencyToAPI(dep)), nil
}

func (h *StrictHandler) RemoveDependency(ctx context.Context, request RemoveDependencyRequestObject) (RemoveDependencyResponseObject, error) {
	actor := getActorFromRequest(ctx)

	if err := h.services.Dependencies.Remove(ctx, string(request.IssueID), string(request.DependsOnID), actor); err != nil {
		if strings.Contains(err.Error(), "not found") {
			return RemoveDependency404JSONResponse{NotFoundJSONResponse{Error: err.Error()}}, nil
		}
		return RemoveDependency500JSONResponse{InternalErrorJSONResponse{Error: err.Error()}}, nil
	}

	return RemoveDependency204Response{}, nil
}

// ====================
// Label handlers
// ====================

func (h *StrictHandler) ListLabels(ctx context.Context, request ListLabelsRequestObject) (ListLabelsResponseObject, error) {
	labels, err := h.services.Labels.List(ctx, string(request.WorkspaceID))
	if err != nil {
		return ListLabels500JSONResponse{InternalErrorJSONResponse{Error: err.Error()}}, nil
	}

	result := make(ListLabels200JSONResponse, len(labels))
	for i, l := range labels {
		result[i] = labelToAPI(l)
	}

	return result, nil
}

func (h *StrictHandler) CreateLabel(ctx context.Context, request CreateLabelRequestObject) (CreateLabelResponseObject, error) {
	body := request.Body
	color := ""
	desc := ""
	if body.Color != nil {
		color = *body.Color
	}
	if body.Description != nil {
		desc = *body.Description
	}

	label, err := h.services.Labels.Create(ctx, string(request.WorkspaceID), body.Name, color, desc)
	if err != nil {
		if strings.Contains(err.Error(), "required") {
			return CreateLabel400JSONResponse{BadRequestJSONResponse{Error: err.Error()}}, nil
		}
		return CreateLabel500JSONResponse{InternalErrorJSONResponse{Error: err.Error()}}, nil
	}

	return CreateLabel201JSONResponse(labelToAPI(label)), nil
}

func (h *StrictHandler) UpdateLabel(ctx context.Context, request UpdateLabelRequestObject) (UpdateLabelResponseObject, error) {
	body := request.Body

	label, err := h.services.Labels.Update(ctx, string(request.WorkspaceID), string(request.LabelName), body.Color, body.Description)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return UpdateLabel404JSONResponse{NotFoundJSONResponse{Error: err.Error()}}, nil
		}
		return UpdateLabel500JSONResponse{InternalErrorJSONResponse{Error: err.Error()}}, nil
	}

	return UpdateLabel200JSONResponse(labelToAPI(label)), nil
}

func (h *StrictHandler) DeleteLabel(ctx context.Context, request DeleteLabelRequestObject) (DeleteLabelResponseObject, error) {
	if err := h.services.Labels.Delete(ctx, string(request.WorkspaceID), string(request.LabelName)); err != nil {
		if strings.Contains(err.Error(), "not found") {
			return DeleteLabel404JSONResponse{NotFoundJSONResponse{Error: err.Error()}}, nil
		}
		return DeleteLabel500JSONResponse{InternalErrorJSONResponse{Error: err.Error()}}, nil
	}

	return DeleteLabel204Response{}, nil
}

func (h *StrictHandler) AddLabelToIssue(ctx context.Context, request AddLabelToIssueRequestObject) (AddLabelToIssueResponseObject, error) {
	actor := getActorFromRequest(ctx)

	if err := h.services.Labels.AddToIssue(ctx, string(request.IssueID), request.Body.Label, actor); err != nil {
		if strings.Contains(err.Error(), "not found") {
			return AddLabelToIssue404JSONResponse{NotFoundJSONResponse{Error: err.Error()}}, nil
		}
		return AddLabelToIssue500JSONResponse{InternalErrorJSONResponse{Error: err.Error()}}, nil
	}

	return AddLabelToIssue204Response{}, nil
}

func (h *StrictHandler) RemoveLabelFromIssue(ctx context.Context, request RemoveLabelFromIssueRequestObject) (RemoveLabelFromIssueResponseObject, error) {
	actor := getActorFromRequest(ctx)

	if err := h.services.Labels.RemoveFromIssue(ctx, string(request.IssueID), string(request.LabelName), actor); err != nil {
		if strings.Contains(err.Error(), "not found") {
			return RemoveLabelFromIssue404JSONResponse{NotFoundJSONResponse{Error: err.Error()}}, nil
		}
		return RemoveLabelFromIssue500JSONResponse{InternalErrorJSONResponse{Error: err.Error()}}, nil
	}

	return RemoveLabelFromIssue204Response{}, nil
}

// ====================
// Comment handlers
// ====================

func (h *StrictHandler) GetComments(ctx context.Context, request GetCommentsRequestObject) (GetCommentsResponseObject, error) {
	comments, err := h.services.Comments.List(ctx, string(request.IssueID))
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return GetComments404JSONResponse{NotFoundJSONResponse{Error: err.Error()}}, nil
		}
		return GetComments500JSONResponse{InternalErrorJSONResponse{Error: err.Error()}}, nil
	}

	result := make(GetComments200JSONResponse, len(comments))
	for i, c := range comments {
		result[i] = commentToAPI(c)
	}

	return result, nil
}

func (h *StrictHandler) AddComment(ctx context.Context, request AddCommentRequestObject) (AddCommentResponseObject, error) {
	actor := getActorFromRequest(ctx)

	comment, err := h.services.Comments.Add(ctx, string(request.IssueID), actor, request.Body.Text)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return AddComment404JSONResponse{NotFoundJSONResponse{Error: err.Error()}}, nil
		}
		if strings.Contains(err.Error(), "required") {
			return AddComment400JSONResponse{BadRequestJSONResponse{Error: err.Error()}}, nil
		}
		return AddComment500JSONResponse{InternalErrorJSONResponse{Error: err.Error()}}, nil
	}

	return AddComment201JSONResponse(commentToAPI(comment)), nil
}

func (h *StrictHandler) UpdateComment(ctx context.Context, request UpdateCommentRequestObject) (UpdateCommentResponseObject, error) {
	if err := h.services.Comments.Update(ctx, request.CommentID, request.Body.Text); err != nil {
		if strings.Contains(err.Error(), "not found") {
			return UpdateComment404JSONResponse{NotFoundJSONResponse{Error: err.Error()}}, nil
		}
		if strings.Contains(err.Error(), "required") {
			return UpdateComment400JSONResponse{BadRequestJSONResponse{Error: err.Error()}}, nil
		}
		return UpdateComment500JSONResponse{InternalErrorJSONResponse{Error: err.Error()}}, nil
	}

	return UpdateComment204Response{}, nil
}

func (h *StrictHandler) DeleteComment(ctx context.Context, request DeleteCommentRequestObject) (DeleteCommentResponseObject, error) {
	if err := h.services.Comments.Delete(ctx, request.CommentID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			return DeleteComment404JSONResponse{NotFoundJSONResponse{Error: err.Error()}}, nil
		}
		return DeleteComment500JSONResponse{InternalErrorJSONResponse{Error: err.Error()}}, nil
	}

	return DeleteComment204Response{}, nil
}

func (h *StrictHandler) GetEvents(ctx context.Context, request GetEventsRequestObject) (GetEventsResponseObject, error) {
	limit := 50
	if request.Params.Limit != nil {
		limit = *request.Params.Limit
	}

	events, err := h.services.Comments.GetEvents(ctx, string(request.IssueID), limit)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return GetEvents404JSONResponse{NotFoundJSONResponse{Error: err.Error()}}, nil
		}
		return GetEvents500JSONResponse{InternalErrorJSONResponse{Error: err.Error()}}, nil
	}

	result := make(GetEvents200JSONResponse, len(events))
	for i, e := range events {
		result[i] = eventToAPI(e)
	}

	return result, nil
}

// ====================
// Type conversion helpers
// ====================

func workspaceToAPI(ws *types.Workspace) Workspace {
	return Workspace{
		ID:          ws.ID,
		Name:        ws.Name,
		Path:        ptrString(ws.Path),
		Description: ptrString(ws.Description),
		Prefix:      ws.Prefix,
		CreatedAt:   ws.CreatedAt,
		UpdatedAt:   ws.UpdatedAt,
	}
}

func statisticsToAPI(s *types.Statistics) Statistics {
	return Statistics{
		WorkspaceID:      s.WorkspaceID,
		TotalIssues:      s.TotalIssues,
		OpenIssues:       s.OpenIssues,
		InProgressIssues: s.InProgressIssues,
		ClosedIssues:     s.ClosedIssues,
		BlockedIssues:    s.BlockedIssues,
		DeferredIssues:   s.DeferredIssues,
		ReadyIssues:      s.ReadyIssues,
		AvgLeadTimeHours: ptrFloat64(s.AvgLeadTimeHours),
	}
}

func issueToAPI(i *types.Issue) Issue {
	return Issue{
		ID:                 i.ID,
		WorkspaceID:        i.WorkspaceID,
		Title:              i.Title,
		Description:        ptrString(i.Description),
		AcceptanceCriteria: ptrString(i.AcceptanceCriteria),
		Notes:              ptrString(i.Notes),
		Status:             Status(i.Status),
		Priority:           i.Priority,
		IssueType:          IssueType(i.IssueType),
		Assignee:           ptrString(i.Assignee),
		ExternalRef:        ptrString(i.ExternalRef),
		CreatedAt:          i.CreatedAt,
		UpdatedAt:          i.UpdatedAt,
		ClosedAt:           i.ClosedAt,
		CloseReason:        ptrString(i.CloseReason),
	}
}

func issueDetailsToAPI(d *types.IssueDetails) Issue {
	issue := issueToAPI(&d.Issue)

	if len(d.Labels) > 0 {
		issue.Labels = &d.Labels
	}

	if len(d.Dependencies) > 0 {
		deps := make([]Dependency, len(d.Dependencies))
		for i, dep := range d.Dependencies {
			deps[i] = dependencyToAPI(dep)
		}
		issue.Dependencies = &deps
	}

	if len(d.Comments) > 0 {
		comments := make([]Comment, len(d.Comments))
		for i, c := range d.Comments {
			comments[i] = commentToAPI(c)
		}
		issue.Comments = &comments
	}

	return issue
}

func blockedIssueToAPI(b *types.BlockedIssue) BlockedIssue {
	return BlockedIssue{
		ID:                 b.ID,
		WorkspaceID:        b.WorkspaceID,
		Title:              b.Title,
		Description:        ptrString(b.Description),
		AcceptanceCriteria: ptrString(b.AcceptanceCriteria),
		Notes:              ptrString(b.Notes),
		Status:             Status(b.Status),
		Priority:           b.Priority,
		IssueType:          IssueType(b.IssueType),
		Assignee:           ptrString(b.Assignee),
		ExternalRef:        ptrString(b.ExternalRef),
		CreatedAt:          b.CreatedAt,
		UpdatedAt:          b.UpdatedAt,
		ClosedAt:           b.ClosedAt,
		CloseReason:        ptrString(b.CloseReason),
		BlockedByCount:     b.BlockedByCount,
		BlockedBy:          b.BlockedBy,
	}
}

func dependencyToAPI(d *types.Dependency) Dependency {
	return Dependency{
		IssueID:     d.IssueID,
		DependsOnID: d.DependsOnID,
		Type:        DependencyType(d.Type),
		CreatedAt:   d.CreatedAt,
		CreatedBy:   ptrString(d.CreatedBy),
	}
}

func labelToAPI(l *types.Label) Label {
	return Label{
		WorkspaceID: l.WorkspaceID,
		Name:        l.Name,
		Color:       ptrString(l.Color),
		Description: ptrString(l.Description),
	}
}

func commentToAPI(c *types.Comment) Comment {
	return Comment{
		ID:        c.ID,
		IssueID:   c.IssueID,
		Author:    c.Author,
		Text:      c.Text,
		CreatedAt: c.CreatedAt,
		UpdatedAt: ptrTime(c.UpdatedAt),
	}
}

func eventToAPI(e *types.Event) Event {
	return Event{
		ID:        e.ID,
		IssueID:   e.IssueID,
		EventType: EventType(e.EventType),
		Actor:     e.Actor,
		OldValue:  e.OldValue,
		NewValue:  e.NewValue,
		Comment:   e.Comment,
		CreatedAt: e.CreatedAt,
	}
}

// Pointer helpers
func ptrString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func ptrInt(i int) *int {
	return &i
}

func ptrFloat64(f float64) *float64 {
	if f == 0 {
		return nil
	}
	return &f
}

func ptrTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

// Ensure StrictHandler implements StrictServerInterface
var _ StrictServerInterface = (*StrictHandler)(nil)
