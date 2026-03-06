// @ts-nocheck
import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'fs';
import { resolve } from 'path';

const componentSource = readFileSync(
	resolve(import.meta.dir, 'FileTree.svelte'),
	'utf-8'
);

describe('FileTree component', () => {
	test('button element has title={name} for tooltip', () => {
		// The button for each file entry should have a title attribute showing the full path
		expect(componentSource).toContain('title={name}');
	});

	test('filename span uses whitespace-nowrap instead of truncate', () => {
		// The filename span should not truncate text
		expect(componentSource).not.toMatch(/class="[^"]*truncate[^"]*".*\{name\}/);
		// It should use whitespace-nowrap to prevent wrapping
		expect(componentSource).toMatch(/class="[^"]*whitespace-nowrap[^"]*".*\{name\}/s);
	});

	test('scrollable container has both overflow-y-auto and overflow-x-auto', () => {
		// The file list container should scroll both vertically and horizontally
		expect(componentSource).toMatch(/overflow-y-auto/);
		expect(componentSource).toMatch(/overflow-x-auto/);
		// Both should be on the same element
		expect(componentSource).toMatch(/class="[^"]*overflow-y-auto[^"]*overflow-x-auto[^"]*"/);
	});
});
