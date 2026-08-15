import { describe, expect, test } from 'vitest';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, resolve } from 'node:path';

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);
const componentSource = readFileSync(resolve(__dirname, 'FloatingToolbar.svelte'), 'utf-8');

describe('FloatingToolbar component', () => {
	test('accepts anchorRect, onComment, onDismiss props', () => {
		expect(componentSource).toMatch(/anchorRect/);
		expect(componentSource).toMatch(/onComment/);
		expect(componentSource).toMatch(/onDismiss/);
	});

	test('imports positioning helper without a .ts extension', () => {
		expect(componentSource).toMatch(/from ['"]\.\/positioning['"]/);
		expect(componentSource).not.toMatch(/from ['"]\.\/positioning\.ts['"]/);
	});

	test('has no reviewer name-capture code', () => {
		expect(componentSource).not.toMatch(/reviewerName/);
		expect(componentSource).not.toMatch(/onSetName/);
		expect(componentSource).not.toMatch(/nameDraft/);
		expect(componentSource).not.toMatch(/nameError/);
		expect(componentSource).not.toMatch(/needsName/);
		expect(componentSource).not.toMatch(/nameReady/);
		expect(componentSource).not.toMatch(/commitName/);
		expect(componentSource).not.toMatch(/flashRequired/);
		expect(componentSource).not.toMatch(/handleNameKey/);
		expect(componentSource).not.toMatch(/tryAction/);
	});

	test('has no praise/delete/suggest/quick-label taxonomy', () => {
		expect(componentSource).not.toMatch(/ToolbarAction/);
		expect(componentSource).not.toMatch(/praise/i);
		expect(componentSource).not.toMatch(/quick-label/);
		expect(componentSource).not.toMatch(/suggest/i);
		expect(componentSource).not.toMatch(/\bdelete\b/i);
	});

	test('renders exactly one button, the Comment pill', () => {
		const buttonMatches = componentSource.match(/<button\b/g) ?? [];
		expect(buttonMatches.length).toBe(1);
		expect(componentSource).toMatch(/aria-label="Comment on selection"/);
		expect(componentSource).toMatch(/onclick=\{onComment\}/);
		expect(componentSource).toMatch(/>\s*Comment\s*</);
	});

	test('computePosition uses pure viewport coordinates (no window.scrollX/scrollY)', () => {
		expect(componentSource).toMatch(/function computePosition/);
		expect(componentSource).not.toMatch(/window\.scrollX/);
		expect(componentSource).not.toMatch(/window\.scrollY/);
		expect(componentSource).toMatch(/clampedAnchorLeft/);
	});

	test('measures toolbar width on mount, seeded at 220', () => {
		expect(componentSource).toMatch(/measuredWidth = \$state\(220\)/);
	});

	test('dismisses on Escape and outside mousedown', () => {
		expect(componentSource).toMatch(/e\.key === ['"]Escape['"]/);
		expect(componentSource).toMatch(/mousedown/);
	});

	test('dismisses on window scroll with a capture+passive listener', () => {
		expect(componentSource).toMatch(
			/addEventListener\(['"]scroll['"],\s*handleScroll,\s*\{\s*capture:\s*true,\s*passive:\s*true\s*\}\)/
		);
		expect(componentSource).toMatch(/removeEventListener\(['"]scroll['"],\s*handleScroll/);
	});

	test('uses Svelte 5 runes', () => {
		expect(componentSource).toMatch(/\$state|\$derived|\$props/);
	});
});
