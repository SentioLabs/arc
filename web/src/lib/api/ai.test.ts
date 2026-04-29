// @ts-nocheck
import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

const apiSource = readFileSync(resolve(import.meta.dir, 'ai.ts'), 'utf-8');

describe('AI API client - project-scoped paths', () => {
	test('listAISessions accepts projectId as first parameter', () => {
		expect(apiSource).toMatch(/function listAISessions\(\s*\n?\s*projectId:\s*string/);
	});

	test('listAISessions uses project-scoped API path', () => {
		expect(apiSource).toContain("'/projects/{projectId}/ai/sessions'");
	});

	test('getAISession accepts projectId as first parameter', () => {
		expect(apiSource).toMatch(/function getAISession\(\s*\n?\s*projectId:\s*string/);
	});

	test('getAISession uses project-scoped API path', () => {
		expect(apiSource).toContain("'/projects/{projectId}/ai/sessions/{sessionId}'");
	});

	test('exports getAIAgent function with projectId', () => {
		expect(apiSource).toContain('export async function getAIAgent(');
		expect(apiSource).toMatch(/function getAIAgent\(\s*\n?\s*projectId:\s*string/);
	});

	test('getAIAgent calls correct project-scoped API endpoint', () => {
		expect(apiSource).toContain("'/projects/{projectId}/ai/sessions/{sessionId}/agents/{agentId}'");
		expect(apiSource).toContain('path: { projectId, sessionId, agentId }');
	});

	test('exports getAgentTranscript function with projectId', () => {
		expect(apiSource).toContain('export async function getAgentTranscript(');
		expect(apiSource).toMatch(/function getAgentTranscript\(\s*\n?\s*projectId:\s*string/);
	});

	test('getAgentTranscript fetches from project-scoped transcript endpoint', () => {
		expect(apiSource).toContain('encodeURIComponent(projectId)}/ai/sessions/');
		expect(apiSource).toContain('/agents/');
		expect(apiSource).toContain('/transcript');
	});

	test('getAgentTranscript returns an array', () => {
		expect(apiSource).toMatch(/return\s+(data|entries|await)/);
	});

	test('deleteAISession accepts projectId as first parameter', () => {
		expect(apiSource).toMatch(/function deleteAISession\(\s*projectId:\s*string/);
	});

	test('deleteAISession uses project-scoped API path', () => {
		expect(apiSource).toContain("api.DELETE('/projects/{projectId}/ai/sessions/{sessionId}'");
	});

	test('batchDeleteAISessions accepts projectId as first parameter', () => {
		expect(apiSource).toMatch(/function batchDeleteAISessions\(\s*\n?\s*projectId:\s*string/);
	});

	test('batchDeleteAISessions uses project-scoped URL', () => {
		expect(apiSource).toContain('encodeURIComponent(projectId)}/ai/sessions/batch-delete');
	});

	test('listAIAgents accepts projectId as first parameter', () => {
		expect(apiSource).toMatch(/function listAIAgents\(\s*\n?\s*projectId:\s*string/);
	});

	test('listAIAgents uses project-scoped API path', () => {
		expect(apiSource).toContain("'/projects/{projectId}/ai/sessions/{sessionId}/agents'");
	});

	test('no old non-project-scoped paths remain', () => {
		const lines = apiSource.split('\n');
		for (const line of lines) {
			if (line.trim().startsWith('//') || line.trim().startsWith('*')) continue;
			if (line.includes("'/ai/sessions")) {
				throw new Error(`Found non-project-scoped path: ${line.trim()}`);
			}
		}
	});
});
