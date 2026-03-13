/**
 * Client-side API functions for AI session tracking.
 */

import { api } from './client';
import type { components } from './types';

export type AISessionResponse = components['schemas']['AISessionResponse'];
export type AIAgentResponse = components['schemas']['AIAgentResponse'];

export async function listAISessions(
	limit?: number,
	offset?: number
): Promise<AISessionResponse[]> {
	const { data, error } = await api.GET('/ai/sessions', {
		params: {
			query: { limit, offset }
		}
	});
	if (error) {
		if (typeof error === 'object' && error !== null && 'error' in error) {
			throw new Error(String((error as { error: string }).error));
		}
		throw new Error('Failed to list AI sessions');
	}
	return data ?? [];
}

export async function getAISession(sessionId: string): Promise<AISessionResponse> {
	const { data, error } = await api.GET('/ai/sessions/{sessionId}', {
		params: { path: { sessionId } }
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

export async function listAIAgents(sessionId: string): Promise<AIAgentResponse[]> {
	const { data, error } = await api.GET('/ai/sessions/{sessionId}/agents', {
		params: { path: { sessionId } }
	});
	if (error) {
		if (typeof error === 'object' && error !== null && 'error' in error) {
			throw new Error(String((error as { error: string }).error));
		}
		throw new Error('Failed to list AI agents');
	}
	return data ?? [];
}
