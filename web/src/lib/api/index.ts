/**
 * Client-side API functions for SPA mode.
 * These fetch from the Go API server at runtime.
 */

import { api } from './client';
import type { components } from './types';

export type Workspace = components['schemas']['Workspace'];
export type Statistics = components['schemas']['Statistics'];
export type Issue = components['schemas']['Issue'];
export type PaginatedIssues = components['schemas']['PaginatedIssues'];
export type BlockedIssue = components['schemas']['BlockedIssue'];
export type Label = components['schemas']['Label'];
export type Comment = components['schemas']['Comment'];
export type Event = components['schemas']['Event'];
export type DependencyGraph = components['schemas']['DependencyGraph'];

// Error helper - extracts message from API error response { error: "message" }
function handleError(error: unknown): never {
	if (error instanceof Error) throw error;
	if (typeof error === 'object' && error !== null && 'error' in error) {
		throw new Error(String((error as { error: string }).error));
	}
	if (typeof error === 'string') throw new Error(error);
	throw new Error('An unexpected error occurred');
}

// Workspace APIs
export async function listWorkspaces(): Promise<Workspace[]> {
	const { data, error } = await api.GET('/workspaces');
	if (error) handleError(error);
	return data ?? [];
}

export async function getWorkspace(workspaceId: string): Promise<Workspace> {
	const { data, error } = await api.GET('/workspaces/{workspaceId}', {
		params: { path: { workspaceId } }
	});
	if (error) handleError(error);
	if (!data) throw new Error('Workspace not found');
	return data;
}

export async function getWorkspaceStats(workspaceId: string): Promise<Statistics> {
	const { data, error } = await api.GET('/workspaces/{workspaceId}/stats', {
		params: { path: { workspaceId } }
	});
	if (error) handleError(error);
	if (!data) throw new Error('Failed to load stats');
	return data;
}

export async function deleteWorkspace(workspaceId: string): Promise<void> {
	const { error } = await api.DELETE('/workspaces/{workspaceId}', {
		params: { path: { workspaceId } }
	});
	if (error) handleError(error);
}

export async function deleteWorkspaces(workspaceIds: string[]): Promise<void> {
	await Promise.all(workspaceIds.map((id) => deleteWorkspace(id)));
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
	workspaceId: string,
	filters: IssueFilters = {}
): Promise<PaginatedIssues> {
	const { data, error } = await api.GET('/workspaces/{workspaceId}/issues', {
		params: {
			path: { workspaceId },
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
	workspaceId: string,
	issueId: string,
	details = false
): Promise<Issue> {
	const { data, error } = await api.GET('/workspaces/{workspaceId}/issues/{issueId}', {
		params: {
			path: { workspaceId, issueId },
			query: { details }
		}
	});
	if (error) handleError(error);
	if (!data) throw new Error('Issue not found');
	return data;
}

export type CreateIssueRequest = components['schemas']['CreateIssueRequest'];

export async function createIssue(
	workspaceId: string,
	request: CreateIssueRequest
): Promise<Issue> {
	const { data, error } = await api.POST('/workspaces/{workspaceId}/issues', {
		params: { path: { workspaceId } },
		body: request
	});
	if (error) handleError(error);
	if (!data) throw new Error('Failed to create issue');
	return data;
}

export type UpdateIssueRequest = components['schemas']['UpdateIssueRequest'];

export async function updateIssue(
	workspaceId: string,
	issueId: string,
	request: UpdateIssueRequest
): Promise<Issue> {
	const { data, error } = await api.PUT('/workspaces/{workspaceId}/issues/{issueId}', {
		params: { path: { workspaceId, issueId } },
		body: request
	});
	if (error) handleError(error);
	if (!data) throw new Error('Failed to update issue');
	return data;
}

export async function getReadyWork(
	workspaceId: string,
	filters: { type?: string; priority?: number; assignee?: string; limit?: number } = {}
): Promise<Issue[]> {
	const { data, error } = await api.GET('/workspaces/{workspaceId}/ready', {
		params: {
			path: { workspaceId },
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

export async function getBlockedIssues(workspaceId: string, limit = 50): Promise<BlockedIssue[]> {
	const { data, error } = await api.GET('/workspaces/{workspaceId}/blocked', {
		params: {
			path: { workspaceId },
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

// Comment APIs
export async function getComments(workspaceId: string, issueId: string): Promise<Comment[]> {
	const { data, error } = await api.GET('/workspaces/{workspaceId}/issues/{issueId}/comments', {
		params: { path: { workspaceId, issueId } }
	});
	if (error) handleError(error);
	return data ?? [];
}

// Event APIs
export async function getEvents(
	workspaceId: string,
	issueId: string,
	limit = 50
): Promise<Event[]> {
	const { data, error } = await api.GET('/workspaces/{workspaceId}/issues/{issueId}/events', {
		params: {
			path: { workspaceId, issueId },
			query: { limit }
		}
	});
	if (error) handleError(error);
	return data ?? [];
}

// Dependency APIs
export async function getDependencies(
	workspaceId: string,
	issueId: string
): Promise<DependencyGraph> {
	const { data, error } = await api.GET('/workspaces/{workspaceId}/issues/{issueId}/deps', {
		params: { path: { workspaceId, issueId } }
	});
	if (error) handleError(error);
	return data ?? { dependencies: [], dependents: [] };
}
