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
export type Workspace = components['schemas']['Workspace'];
export type CreateWorkspaceRequest = components['schemas']['CreateWorkspaceRequest'];

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

export async function deleteProject(projectId: string): Promise<void> {
	const { error } = await api.DELETE('/projects/{projectId}', {
		params: { path: { projectId } }
	});
	if (error) handleError(error);
}

export async function deleteProjects(projectIds: string[]): Promise<void> {
	await Promise.all(projectIds.map((id) => deleteProject(id)));
}

export type MergeResult = components['schemas']['MergeResult'];

export async function mergeProjects(targetId: string, sourceIds: string[]): Promise<MergeResult> {
	const { data, error } = await api.POST('/projects/merge', {
		body: { target_id: targetId, source_ids: sourceIds }
	});
	if (error) handleError(error);
	if (!data) throw new Error('Failed to merge projects');
	return data;
}

// Workspace APIs (directory paths)
export async function listWorkspaces(projectId: string): Promise<Workspace[]> {
	const { data, error } = await api.GET('/projects/{projectId}/workspaces', {
		params: { path: { projectId } }
	});
	if (error) handleError(error);
	return data ?? [];
}

export async function createWorkspace(
	projectId: string,
	request: CreateWorkspaceRequest
): Promise<Workspace> {
	const { data, error } = await api.POST('/projects/{projectId}/workspaces', {
		params: { path: { projectId } },
		body: request
	});
	if (error) handleError(error);
	if (!data) throw new Error('Failed to create workspace');
	return data;
}

export async function deleteWorkspace(projectId: string, pathId: string): Promise<void> {
	const { error } = await api.DELETE('/projects/{projectId}/workspaces/{pathId}', {
		params: { path: { projectId, pathId } }
	});
	if (error) handleError(error);
}

// Issue APIs
export interface IssueFilters {
	status?: string;
	type?: string;
	priority?: number;
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
				status: filters.status as
					| 'open'
					| 'in_progress'
					| 'blocked'
					| 'deferred'
					| 'closed'
					| undefined,
				type: filters.type as 'bug' | 'feature' | 'task' | 'epic' | 'chore' | undefined,
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
	return data ?? { workspace: projectId, roles: {}, unassigned: [] };
}
