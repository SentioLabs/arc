// @ts-nocheck
import { describe, expect, test } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

const pageSource = readFileSync(resolve(import.meta.dirname, '+page.svelte'), 'utf-8');

describe('Issues list page type filter', () => {
	test('offers a release option', () => {
		expect(pageSource).toContain("{ value: 'release', label: 'Release' }");
	});

	test('offers a milestone option', () => {
		expect(pageSource).toContain("{ value: 'milestone', label: 'Milestone' }");
	});
});
