// @ts-nocheck
import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

const apiSource = readFileSync(resolve(import.meta.dir, 'ai.ts'), 'utf-8');

describe('AI API client', () => {
	test('exports getAIAgent function', () => {
		expect(apiSource).toContain('export async function getAIAgent(');
	});

	test('getAIAgent calls correct API endpoint with sessionId and agentId', () => {
		expect(apiSource).toContain("'/ai/sessions/{sessionId}/agents/{agentId}'");
		expect(apiSource).toContain('path: { sessionId, agentId }');
	});

	test('exports getAgentTranscript function', () => {
		expect(apiSource).toContain('export async function getAgentTranscript(');
	});

	test('getAgentTranscript fetches from the correct transcript endpoint', () => {
		// The transcript endpoint is not in the OpenAPI spec, so it uses fetch directly
		expect(apiSource).toContain('/api/v1/ai/sessions/');
		expect(apiSource).toContain('/agents/');
		expect(apiSource).toContain('/transcript');
	});

	test('getAgentTranscript returns an array', () => {
		// Should return the parsed JSON (array of transcript entries)
		expect(apiSource).toMatch(/return\s+(data|entries|await)/);
	});
});
