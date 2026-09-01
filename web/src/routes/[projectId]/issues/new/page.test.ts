// @ts-nocheck
import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

const pageSource = readFileSync(resolve(import.meta.dir, '+page.svelte'), 'utf-8');

describe('New issue page type select', () => {
	test('issueType state accepts release and milestone', () => {
		expect(pageSource).toContain(
			"$state<'bug' | 'feature' | 'task' | 'epic' | 'chore' | 'release' | 'milestone'>("
		);
	});

	test('offers a release option', () => {
		expect(pageSource).toContain("{ value: 'release', label: 'Release' }");
	});

	test('offers a milestone option', () => {
		expect(pageSource).toContain("{ value: 'milestone', label: 'Milestone' }");
	});
});
