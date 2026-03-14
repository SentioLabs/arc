/**
 * Client-side API functions for SPA mode.
 * These fetch from the Go API server at runtime.
 */

import { api } from './client';
import type { components } from './types';

export type Project = components['schemas']['Project'];
export type Statistics = components['schemas']['Statistics'];
export type Issue = components['schemas']['Issue'];
export type PaginatedIssues = components['schemas']['PaginatedIssues'];
export type BlockedIssue = components['schemas']['BlockedIssue'];
export type Label = components['schemas']['Label'];
export type Comment = components['schemas']['Comment'];
export type Event = components['schemas']['Event'];
export type DependencyGraph = components['schemas']['DependencyGraph'];
export type TeamContext = components['schemas']['TeamContext'];
export type TeamContextIssue = components['schemas']['TeamContextIssue'];
export type TeamContextRole = components['schemas']['TeamContextRole'];
export type TeamContextEpic = components['schemas']['TeamContextEpic'];
export type AddCommentRequest = components['schemas']['AddCommentRequest'];
export type AddLabelToIssueRequest = components['schemas']['AddLabelToIssueRequest'];

// Workspace path types (not in OpenAPI spec — custom routes)
export interface Workspace {
	id: string;
	project_id: string;
	path: string;
	label?: string;
	hostname?: string;
	git_remote?: string;
	path_type?: string;
	last_accessed_at?: string;
	created_at: string;
	updated_at: string;
}

export interface CreateWorkspaceRequest {
	path: string;
	label?: string;
	hostname?: string;
	git_remote?: string;
	path_type?: string;
}

// Error helper - extracts message from API error response { error: "message" }
function handleError(error: unknown): never {
	if (error instanceof Error) throw error;
	if (typeof error === 'object' && error !== null && 'error' in error) {
		throw new Error(String((error as { error: string }).error));
	}
	if (typeof error === 'string') throw new Error(error);
	throw new Error('An unexpected error occurred');
}

// Project APIs
export async function listProjects(): Promise<Project[]> {
	const { data, error } = await api.GET('/projects');
	if (error) handleError(error);
	return data ?? [];
}

export async function getProject(projectId: string): Promise<Project> {
	const { data, error } = await api.GET('/projects/{projectId}', {
		params: { path: { projectId } }
	});
	if (error) handleError(error);
	if (!data) throw new Error('Project not found');
	return data;
}

export async function getProjectStats(projectId: string): Promise<Statistics> {
	const { data, error } = await api.GET('/projects/{projectId}/stats', {
		params: { path: { projectId } }
	});
	if (error) handleError(error);
	if (!data) throw new Error('Failed to load stats');
	return data;
}

export async function updateProject(
	projectId: string,
	updates: { name?: string; description?: string }
): Promise<Project> {
	const { data, error } = await api.PUT('/projects/{projectId}', {
		params: { path: { projectId } },
		body: updates
	});
	if (error) handleError(error);
	if (!data) throw new Error('Failed to update project');
	return data;
}

export async function deleteProject(projectId: string): Promise<void> {
	const { error } = await api.DELETE('/projects/{projectId}', {
		params: { path: { projectId } }
	});
	if (error) handleError(error);
}

export async function deleteProjects(projectIds: string[]): Promise<void> {
	await Promise.all(projectIds.map((id) => deleteProject(id)));
}

export interface MergeResult {
	target_project: Project;
	issues_moved: number;
	plans_moved: number;
	sources_deleted: string[];
}

export async function mergeProjects(targetId: string, sourceIds: string[]): Promise<MergeResult> {
	const response = await fetch('/api/v1/projects/merge', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ target_id: targetId, source_ids: sourceIds })
	});
	if (!response.ok) {
		const body = await response.json().catch(() => ({ error: 'Failed to merge projects' }));
		handleError(body);
	}
	return response.json();
}

// Workspace APIs (directory paths — not in OpenAPI spec)
export async function listWorkspaces(projectId: string): Promise<Workspace[]> {
	const response = await fetch(`/api/v1/projects/${projectId}/workspaces`);
	if (!response.ok) {
		const body = await response.json().catch(() => ({ error: 'Failed to list workspaces' }));
		handleError(body);
	}
	return response.json();
}

export async function createWorkspace(
	projectId: string,
	request: CreateWorkspaceRequest
): Promise<Workspace> {
	const response = await fetch(`/api/v1/projects/${projectId}/workspaces`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(request)
	});
	if (!response.ok) {
		const body = await response.json().catch(() => ({ error: 'Failed to create workspace' }));
		handleError(body);
	}
	return response.json();
}

export async function deleteWorkspace(projectId: string, pathId: string): Promise<void> {
	const response = await fetch(`/api/v1/projects/${projectId}/workspaces/${pathId}`, {
		method: 'DELETE'
	});
	if (!response.ok) {
		const body = await response.json().catch(() => ({ error: 'Failed to delete workspace' }));
		handleError(body);
	}
}

// Issue APIs
export interface IssueFilters {
	status?: string[];
	type?: string[];
	priority?: number[];
	assignee?: string;
	q?: string;
	limit?: number;
	offset?: number;
}

export async function listIssues(
	projectId: string,
	filters: IssueFilters = {}
): Promise<PaginatedIssues> {
	const { data, error } = await api.GET('/projects/{projectId}/issues', {
		params: {
			path: { projectId },
			query: {
				status: filters.status as components['schemas']['Status'][] | undefined,
				type: filters.type as components['schemas']['IssueType'][] | undefined,
				priority: filters.priority,
				assignee: filters.assignee,
				q: filters.q,
				limit: filters.limit,
				offset: filters.offset
			}
		}
	});
	if (error) handleError(error);
	return data ?? { data: [], total: 0 };
}

export async function getIssue(
	projectId: string,
	issueId: string,
	details = false
): Promise<Issue> {
	const { data, error } = await api.GET('/projects/{projectId}/issues/{issueId}', {
		params: {
			path: { projectId, issueId },
			query: { details }
		}
	});
	if (error) handleError(error);
	if (!data) throw new Error('Issue not found');
	return data;
}

export type CreateIssueRequest = components['schemas']['CreateIssueRequest'];

export async function createIssue(
	projectId: string,
	request: CreateIssueRequest
): Promise<Issue> {
	const { data, error } = await api.POST('/projects/{projectId}/issues', {
		params: { path: { projectId } },
		body: request
	});
	if (error) handleError(error);
	if (!data) throw new Error('Failed to create issue');
	return data;
}

export type UpdateIssueRequest = components['schemas']['UpdateIssueRequest'];

export async function updateIssue(
	projectId: string,
	issueId: string,
	request: UpdateIssueRequest
): Promise<Issue> {
	const { data, error } = await api.PUT('/projects/{projectId}/issues/{issueId}', {
		params: { path: { projectId, issueId } },
		body: request
	});
	if (error) handleError(error);
	if (!data) throw new Error('Failed to update issue');
	return data;
}

export async function getReadyWork(
	projectId: string,
	filters: { type?: string; priority?: number; assignee?: string; limit?: number } = {}
): Promise<Issue[]> {
	const { data, error } = await api.GET('/projects/{projectId}/ready', {
		params: {
			path: { projectId },
			query: {
				type: filters.type as 'bug' | 'feature' | 'task' | 'epic' | 'chore' | undefined,
				priority: filters.priority,
				assignee: filters.assignee,
				limit: filters.limit
			}
		}
	});
	if (error) handleError(error);
	return data ?? [];
}

export async function getBlockedIssues(projectId: string, limit = 50): Promise<BlockedIssue[]> {
	const { data, error } = await api.GET('/projects/{projectId}/blocked', {
		params: {
			path: { projectId },
			query: { limit }
		}
	});
	if (error) handleError(error);
	return data ?? [];
}

// Label APIs (global)
export async function listLabels(): Promise<Label[]> {
	const { data, error } = await api.GET('/labels');
	if (error) handleError(error);
	return data ?? [];
}

export async function createLabel(
	name: string,
	color?: string,
	description?: string
): Promise<Label> {
	const { data, error } = await api.POST('/labels', {
		body: { name, color, description }
	});
	if (error) handleError(error);
	if (!data) throw new Error('Failed to create label');
	return data;
}

export async function updateLabel(
	name: string,
	color?: string,
	description?: string
): Promise<Label> {
	const { data, error } = await api.PUT('/labels/{labelName}', {
		params: { path: { labelName: name } },
		body: { color, description }
	});
	if (error) handleError(error);
	if (!data) throw new Error('Failed to update label');
	return data;
}

export async function deleteLabel(name: string): Promise<void> {
	const { error } = await api.DELETE('/labels/{labelName}', {
		params: { path: { labelName: name } }
	});
	if (error) handleError(error);
}

// Issue Label APIs
export async function addLabelToIssue(
	projectId: string,
	issueId: string,
	label: string
): Promise<void> {
	const { error } = await api.POST('/projects/{projectId}/issues/{issueId}/labels', {
		params: { path: { projectId, issueId } },
		body: { label }
	});
	if (error) handleError(error);
}

export async function removeLabelFromIssue(
	projectId: string,
	issueId: string,
	labelName: string
): Promise<void> {
	const { error } = await api.DELETE(
		'/projects/{projectId}/issues/{issueId}/labels/{labelName}',
		{
			params: { path: { projectId, issueId, labelName } }
		}
	);
	if (error) handleError(error);
}

// Comment APIs
export async function getComments(projectId: string, issueId: string): Promise<Comment[]> {
	const { data, error } = await api.GET('/projects/{projectId}/issues/{issueId}/comments', {
		params: { path: { projectId, issueId } }
	});
	if (error) handleError(error);
	return data ?? [];
}

export async function createComment(
	projectId: string,
	issueId: string,
	text: string
): Promise<Comment> {
	const { data, error } = await api.POST(
		'/projects/{projectId}/issues/{issueId}/comments',
		{
			params: { path: { projectId, issueId } },
			body: { text }
		}
	);
	if (error) handleError(error);
	if (!data) throw new Error('Failed to create comment');
	return data;
}

// Event APIs
export async function getEvents(
	projectId: string,
	issueId: string,
	limit = 50
): Promise<Event[]> {
	const { data, error } = await api.GET('/projects/{projectId}/issues/{issueId}/events', {
		params: {
			path: { projectId, issueId },
			query: { limit }
		}
	});
	if (error) handleError(error);
	return data ?? [];
}

// Dependency APIs
export async function getDependencies(
	projectId: string,
	issueId: string
): Promise<DependencyGraph> {
	const { data, error } = await api.GET('/projects/{projectId}/issues/{issueId}/deps', {
		params: { path: { projectId, issueId } }
	});
	if (error) handleError(error);
	return data ?? { dependencies: [], dependents: [] };
}

// Filesystem Browse API
export interface BrowseEntry {
	name: string;
	path: string;
	is_dir: boolean;
	is_git_repo: boolean;
}

export async function browseFilesystem(dir: string): Promise<BrowseEntry[]> {
	const response = await fetch(`/api/v1/filesystem/browse?dir=${encodeURIComponent(dir)}`);
	if (!response.ok) {
		const body = await response.json().catch(() => ({ error: 'Failed to browse filesystem' }));
		handleError(body);
	}
	const data = await response.json();
	return data ?? [];
}

// Plan APIs
export type Plan = components['schemas']['Plan'];

// List all plans across projects (global view)
export async function listAllPlans(status?: string): Promise<Plan[]> {
	const params: Record<string, string> = {};
	if (status) params.status = status;
	const query = new URLSearchParams(params).toString();
	const response = await fetch(`/api/v1/plans${query ? `?${query}` : ''}`);
	if (!response.ok) {
		const body = await response.json().catch(() => ({ error: 'Failed to list plans' }));
		handleError(body);
	}
	return response.json();
}

// List plans for a specific project
export async function listProjectPlans(projectId: string, status?: string): Promise<Plan[]> {
	const { data, error } = await api.GET('/projects/{projectId}/plans', {
		params: {
			path: { projectId },
			query: { status: status as 'draft' | 'approved' | 'rejected' | undefined }
		}
	});
	if (error) handleError(error);
	return data ?? [];
}

// Get a single plan by ID
export async function getPlan(projectId: string, planId: string): Promise<Plan> {
	const { data, error } = await api.GET('/projects/{projectId}/plans/{planId}', {
		params: { path: { projectId, planId } }
	});
	if (error) handleError(error);
	if (!data) throw new Error('Plan not found');
	return data;
}

// Update plan content and optionally link/unlink issue
export async function updatePlan(
	projectId: string,
	planId: string,
	updates: { title?: string; content?: string; issue_id?: string }
): Promise<Plan> {
	const { data, error } = await api.PUT('/projects/{projectId}/plans/{planId}', {
		params: { path: { projectId, planId } },
		body: updates as any
	});
	if (error) handleError(error);
	if (!data) throw new Error('Failed to update plan');
	return data;
}

// Update plan status (draft/approved/rejected)
export async function updatePlanStatus(
	projectId: string,
	planId: string,
	status: 'draft' | 'approved' | 'rejected'
): Promise<Plan> {
	const { data, error } = await api.PATCH('/projects/{projectId}/plans/{planId}/status', {
		params: { path: { projectId, planId } },
		body: { status }
	});
	if (error) handleError(error);
	if (!data) throw new Error('Failed to update plan status');
	return data;
}

// Delete a plan
export async function deletePlan(projectId: string, planId: string): Promise<void> {
	const { error } = await api.DELETE('/projects/{projectId}/plans/{planId}', {
		params: { path: { projectId, planId } }
	});
	if (error) handleError(error);
}

// Team Context APIs
export async function getTeamContext(
	projectId: string,
	epicId?: string
): Promise<TeamContext> {
	const { data, error } = await api.GET('/projects/{projectId}/team-context', {
		params: {
			path: { projectId },
			query: { epic_id: epicId }
		}
	});
	if (error) handleError(error);
	return data ?? { project: projectId, roles: {}, unassigned: [] };
}
