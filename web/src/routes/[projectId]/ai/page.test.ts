// @ts-nocheck
import { describe, expect, test } from 'bun:test';
import { readFileSync, existsSync } from 'node:fs';
import { resolve } from 'node:path';

const dir = resolve(import.meta.dir);

describe('AI sessions list page', () => {
	test('+page.ts loader file exists', () => {
		expect(existsSync(resolve(dir, '+page.ts'))).toBe(true);
	});

	test('+page.svelte file exists', () => {
		expect(existsSync(resolve(dir, '+page.svelte'))).toBe(true);
	});
});

const pageSource = readFileSync(resolve(dir, '+page.svelte'), 'utf-8');

describe('AI sessions list page content', () => {
	test('gets projectId from page params', () => {
		expect(pageSource).toContain('$page.params.projectId');
	});

	test('passes projectId to listAISessions', () => {
		expect(pageSource).toContain('listAISessions(pid, limit, offset)');
	});

	test('passes projectId to deleteAISession', () => {
		expect(pageSource).toContain('deleteAISession(pid, id)');
	});

	test('passes projectId to batchDeleteAISessions', () => {
		expect(pageSource).toContain('batchDeleteAISessions(pid, [...selected])');
	});

	test('uses project-scoped session links', () => {
		expect(pageSource).toContain('/{projectId}/ai/{session.id}');
	});

	test('displays Path column header instead of CWD', () => {
		expect(pageSource).toContain('>Path<');
		expect(pageSource).not.toContain('>CWD<');
	});

	test('shows last 2 path segments', () => {
		expect(pageSource).toContain(".split('/').slice(-2).join('/')");
	});

	test('uses Svelte 5 runes', () => {
		expect(pageSource).toMatch(/\$state/);
		expect(pageSource).toMatch(/\$derived/);
		expect(pageSource).toMatch(/\$effect/);
	});
});
