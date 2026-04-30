// @ts-nocheck
import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

const componentSource = readFileSync(resolve(import.meta.dir, 'RecentAISessions.svelte'), 'utf-8');

describe('RecentAISessions component', () => {
	test('uses Svelte 5 runes for props', () => {
		expect(componentSource).toContain('$props()');
		expect(componentSource).toContain('projectId');
	});

	test('uses $state for reactive state', () => {
		expect(componentSource).toContain('$state');
	});

	test('imports listAISessions from api', () => {
		expect(componentSource).toContain('listAISessions');
	});

	test('calls listAISessions with limit of 5', () => {
		expect(componentSource).toMatch(/listAISessions\(projectId,\s*5/);
	});

	test('sets up 30-second auto-refresh interval', () => {
		expect(componentSource).toContain('30000');
		expect(componentSource).toMatch(/setInterval/);
	});

	test('cleans up interval on destroy', () => {
		expect(componentSource).toMatch(/clearInterval/);
	});

	test('displays section header with title', () => {
		expect(componentSource).toContain('Recent AI Sessions');
	});

	test('has View all link to AI sessions page', () => {
		expect(componentSource).toMatch(/View all/);
		expect(componentSource).toContain('/{projectId}/ai');
	});

	test('truncates session ID with first 4 and last 4 chars', () => {
		// Should show first 4 + ellipsis + last 4 chars
		expect(componentSource).toMatch(/substring|slice/);
	});

	test('has clickable session ID linking to session detail', () => {
		expect(componentSource).toContain('/{projectId}/ai/');
	});

	test('includes timeAgo helper function', () => {
		expect(componentSource).toContain('function timeAgo');
		expect(componentSource).toContain('just now');
		expect(componentSource).toContain('ago');
	});

	test('displays status summary with color-coded counts', () => {
		expect(componentSource).toContain('text-status-active');
		expect(componentSource).toContain('text-status-closed');
		expect(componentSource).toContain('text-status-blocked');
	});

	test('handles missing agent_summary with dash', () => {
		// When agent_summary is null/undefined, show em dash
		expect(componentSource).toMatch(/[—\u2014]/);
	});

	test('shows loading spinner', () => {
		expect(componentSource).toContain('animate-spin');
	});

	test('shows empty state message', () => {
		expect(componentSource).toContain('No AI sessions yet');
	});

	test('has session count badge', () => {
		expect(componentSource).toMatch(/sessions\.length|sessions\.data\.length/);
	});
});
