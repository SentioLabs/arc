// @ts-nocheck
import { describe, expect, test } from 'bun:test';
import { readFileSync, existsSync } from 'node:fs';
import { resolve } from 'node:path';

const dir = resolve(import.meta.dir);

describe('AI session detail page', () => {
	test('+page.ts loader file exists', () => {
		expect(existsSync(resolve(dir, '+page.ts'))).toBe(true);
	});

	test('+page.svelte file exists', () => {
		expect(existsSync(resolve(dir, '+page.svelte'))).toBe(true);
	});
});

const pageSource = readFileSync(resolve(dir, '+page.svelte'), 'utf-8');

describe('AI session detail page content', () => {
	test('gets projectId from page params', () => {
		expect(pageSource).toContain('$page.params.projectId');
	});

	test('passes projectId to getAISession', () => {
		expect(pageSource).toContain('getAISession(pid, id)');
	});

	test('passes projectId to listAIAgents', () => {
		expect(pageSource).toContain('listAIAgents(pid, id)');
	});

	test('uses project-scoped breadcrumb links', () => {
		expect(pageSource).toContain('/{projectId}/ai');
	});

	test('uses project-scoped agent links', () => {
		expect(pageSource).toContain('/{projectId}/ai/{session.id}/agents/{agent.id}');
	});

	test('uses Svelte 5 runes', () => {
		expect(pageSource).toMatch(/\$state/);
		expect(pageSource).toMatch(/\$derived/);
		expect(pageSource).toMatch(/\$effect/);
	});
});
