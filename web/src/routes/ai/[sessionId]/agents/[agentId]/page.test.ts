// @ts-nocheck
import { describe, expect, test } from 'bun:test';
import { readFileSync, existsSync } from 'node:fs';
import { resolve } from 'node:path';

const dir = resolve(import.meta.dir);

describe('Agent transcript page', () => {
	test('+page.ts loader file exists', () => {
		expect(existsSync(resolve(dir, '+page.ts'))).toBe(true);
	});

	test('+page.svelte file exists', () => {
		expect(existsSync(resolve(dir, '+page.svelte'))).toBe(true);
	});
});

const pageSource = readFileSync(resolve(dir, '+page.svelte'), 'utf-8');

describe('Agent transcript page content', () => {
	test('imports TranscriptViewer component', () => {
		expect(pageSource).toContain('TranscriptViewer');
	});

	test('imports API functions for agent and transcript', () => {
		expect(pageSource).toContain('getAIAgent');
		expect(pageSource).toContain('getAgentTranscript');
	});

	test('displays breadcrumb navigation', () => {
		expect(pageSource).toContain('AI Sessions');
		expect(pageSource).toContain('/ai');
	});

	test('displays agent metadata', () => {
		expect(pageSource).toMatch(/description/);
		expect(pageSource).toMatch(/model/);
		expect(pageSource).toMatch(/status/);
		expect(pageSource).toMatch(/duration/i);
		expect(pageSource).toMatch(/token/i);
		expect(pageSource).toMatch(/tool.use/i);
	});

	test('handles loading state', () => {
		expect(pageSource).toContain('loading');
		expect(pageSource).toMatch(/Loading/);
	});

	test('handles error state', () => {
		expect(pageSource).toContain('error');
	});

	test('uses Svelte 5 runes', () => {
		expect(pageSource).toMatch(/\$state/);
		expect(pageSource).toMatch(/\$derived/);
		expect(pageSource).toMatch(/\$effect/);
	});

	test('handles transcript not found with helpful message', () => {
		expect(pageSource).toMatch(/transcript.*not found|no transcript|unavailable/i);
	});
});
