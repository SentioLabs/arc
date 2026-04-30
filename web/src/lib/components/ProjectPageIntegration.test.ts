// @ts-nocheck
import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

const pageSource = readFileSync(
	resolve(import.meta.dir, '../../routes/[projectId]/+page.svelte'),
	'utf-8'
);

describe('Project page AI sessions integration', () => {
	test('imports RecentAISessions component', () => {
		expect(pageSource).toContain(
			"import RecentAISessions from '$lib/components/RecentAISessions.svelte'"
		);
	});

	test('renders RecentAISessions with projectId prop', () => {
		expect(pageSource).toContain('<RecentAISessions projectId={project.id}');
	});

	test('RecentAISessions appears between stats grid and paths section', () => {
		const statsGridEnd = pageSource.indexOf('<!-- Stats Grid -->');
		const recentSessions = pageSource.indexOf('RecentAISessions projectId');
		const pathsSection = pageSource.indexOf('<!-- Paths Section');

		expect(statsGridEnd).toBeGreaterThan(-1);
		expect(recentSessions).toBeGreaterThan(-1);
		expect(pathsSection).toBeGreaterThan(-1);
		expect(recentSessions).toBeGreaterThan(statsGridEnd);
		expect(recentSessions).toBeLessThan(pathsSection);
	});
});
