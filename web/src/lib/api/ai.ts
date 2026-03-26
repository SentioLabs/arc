/**
 * Client-side API functions for AI session tracking.
 */

import { api } from './client';
import type { components } from './types';

export type AISessionResponse = components['schemas']['AISessionResponse'];
export type AIAgentResponse = components['schemas']['AIAgentResponse'];
export type PaginatedAISessions = components['schemas']['PaginatedAISessions'];

export async function listAISessions(
	projectId: string,
	limit?: number,
	offset?: number
): Promise<PaginatedAISessions> {
	const { data, error } = await api.GET('/projects/{projectId}/ai/sessions', {
		params: {
			path: { projectId },
			query: { limit, offset }
		}
	});
	if (error) {
		if (typeof error === 'object' && error !== null && 'error' in error) {
			throw new Error(String((error as { error: string }).error));
		}
		throw new Error('Failed to list AI sessions');
	}
	return data ?? { data: [] };
}

export async function getAISession(
	projectId: string,
	sessionId: string
): Promise<AISessionResponse> {
	const { data, error } = await api.GET('/projects/{projectId}/ai/sessions/{sessionId}', {
		params: { path: { projectId, sessionId } }
	});
	if (error) {
		if (typeof error === 'object' && error !== null && 'error' in error) {
			throw new Error(String((error as { error: string }).error));
		}
		throw new Error('Failed to get AI session');
	}
	if (!data) throw new Error('AI session not found');
	return data;
}

export async function getAIAgent(
	projectId: string,
	sessionId: string,
	agentId: string
): Promise<AIAgentResponse> {
	const { data, error } = await api.GET(
		'/projects/{projectId}/ai/sessions/{sessionId}/agents/{agentId}',
		{
			params: { path: { projectId, sessionId, agentId } }
		}
	);
	if (error) {
		if (typeof error === 'object' && error !== null && 'error' in error) {
			throw new Error(String((error as { error: string }).error));
		}
		throw new Error('Failed to get AI agent');
	}
	if (!data) throw new Error('AI agent not found');
	return data;
}

export async function getAgentTranscript(
	projectId: string,
	sessionId: string,
	agentId: string
): Promise<Record<string, unknown>[]> {
	const response = await fetch(
		`/api/v1/projects/${encodeURIComponent(projectId)}/ai/sessions/${encodeURIComponent(sessionId)}/agents/${encodeURIComponent(agentId)}/transcript`
	);
	if (!response.ok) {
		if (response.status === 404) {
			throw new Error('Agent transcript not found');
		}
		throw new Error('Failed to get agent transcript');
	}
	return await response.json();
}

export async function deleteAISession(projectId: string, sessionId: string): Promise<void> {
	const { error } = await api.DELETE('/projects/{projectId}/ai/sessions/{sessionId}', {
		params: { path: { projectId, sessionId } }
	});
	if (error) {
		if (typeof error === 'object' && error !== null && 'error' in error) {
			throw new Error(String((error as { error: string }).error));
		}
		throw new Error('Failed to delete AI session');
	}
}

export async function batchDeleteAISessions(
	projectId: string,
	ids: string[]
): Promise<{ deleted: number }> {
	const response = await fetch(
		`/api/v1/projects/${encodeURIComponent(projectId)}/ai/sessions/batch-delete`,
		{
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ ids })
		}
	);
	if (!response.ok) {
		throw new Error('Failed to delete sessions');
	}
	return await response.json();
}

export async function listAIAgents(
	projectId: string,
	sessionId: string
): Promise<AIAgentResponse[]> {
	const { data, error } = await api.GET(
		'/projects/{projectId}/ai/sessions/{sessionId}/agents',
		{
			params: { path: { projectId, sessionId } }
		}
	);
	if (error) {
		if (typeof error === 'object' && error !== null && 'error' in error) {
			throw new Error(String((error as { error: string }).error));
		}
		throw new Error('Failed to list AI agents');
	}
	return data ?? [];
}
