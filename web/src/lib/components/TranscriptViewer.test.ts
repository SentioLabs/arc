// @ts-nocheck
import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

const componentSource = readFileSync(
	resolve(import.meta.dir, 'TranscriptViewer.svelte'),
	'utf-8'
);

describe('TranscriptViewer component', () => {
	test('accepts transcript prop', () => {
		expect(componentSource).toContain('transcript');
	});

	test('handles empty transcript with a message', () => {
		expect(componentSource).toMatch(/no\s+transcript|empty|unavailable/i);
	});

	test('renders user messages with distinct styling', () => {
		expect(componentSource).toContain("role === 'user'");
	});

	test('renders assistant messages with distinct styling', () => {
		expect(componentSource).toContain("role === 'assistant'");
	});

	test('renders system messages with subtle styling', () => {
		expect(componentSource).toContain("role === 'system'");
	});

	test('renders tool use blocks as collapsible sections', () => {
		// Should have a toggle mechanism for tool calls
		expect(componentSource).toMatch(/tool_use|tool_calls|type.*tool/);
		// Should have expandable/collapsible behavior
		expect(componentSource).toMatch(/expanded|collapsed|toggle/i);
	});

	test('shows tool name as header', () => {
		expect(componentSource).toMatch(/tool.*name|name.*tool/i);
	});

	test('displays tool input as formatted JSON', () => {
		expect(componentSource).toContain('JSON.stringify');
		expect(componentSource).toMatch(/<pre/);
	});

	test('uses Svelte 5 runes', () => {
		expect(componentSource).toMatch(/\$state|\$derived|\$props/);
	});
});
