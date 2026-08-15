import { describe, expect, test } from 'vitest';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, resolve } from 'node:path';

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);
const componentSource = readFileSync(resolve(__dirname, 'CommentPopover.svelte'), 'utf-8');

describe('CommentPopover component', () => {
	test('accepts the shared props contract', () => {
		expect(componentSource).toMatch(/anchorRect/);
		expect(componentSource).toMatch(/quotedText/);
		expect(componentSource).toMatch(/initialBody = ['"]{2}/);
		expect(componentSource).toMatch(/submitLabel = ['"]Save['"]/);
		expect(componentSource).toMatch(/onSave/);
		expect(componentSource).toMatch(/onCancel/);
	});

	test('imports positioning and platform helpers without .ts extensions', () => {
		expect(componentSource).toMatch(/from ['"]\.\/positioning['"]/);
		expect(componentSource).not.toMatch(/from ['"]\.\/positioning\.ts['"]/);
		expect(componentSource).toMatch(/from ['"]\.\/platform['"]/);
		expect(componentSource).not.toMatch(/from ['"]\.\/platform\.ts['"]/);
	});

	test('has no suggest-mode code', () => {
		expect(componentSource).not.toMatch(/PopoverMode/);
		expect(componentSource).not.toMatch(/\bmode\b/);
		expect(componentSource).not.toMatch(/suggested/i);
		expect(componentSource).not.toMatch(/isSuggest/);
	});

	test('initializes body directly from initialBody, not via $effect', () => {
		expect(componentSource).toMatch(/let body = \$state\(initialBody\)/);
	});

	test('computePosition uses pure viewport coordinates (no window.scrollX/scrollY)', () => {
		expect(componentSource).toMatch(/function computePosition/);
		expect(componentSource).not.toMatch(/window\.scrollX/);
		expect(componentSource).not.toMatch(/window\.scrollY/);
		expect(componentSource).toMatch(/POPOVER_HEIGHT = 120/);
		expect(componentSource).toMatch(/clampedAnchorLeft/);
	});

	test('guards dirty drafts with a confirm on cancel', () => {
		expect(componentSource).toMatch(/const dirty = \$derived/);
		expect(componentSource).toMatch(/function requestCancel/);
		expect(componentSource).toMatch(/window\.confirm\(/);
	});

	test('routes escape, outside click, and scroll dismiss through requestCancel', () => {
		expect(componentSource).toMatch(/e\.key === ['"]Escape['"][\s\S]{0,80}requestCancel\(\)/);
		expect(componentSource).toMatch(/handleDocumentClick/);
		expect(componentSource).toMatch(/function handleScroll\(\)\s*\{\s*requestCancel\(\);?\s*\}/);
	});

	test('dismisses on window scroll with a capture+passive listener', () => {
		expect(componentSource).toMatch(
			/addEventListener\(['"]scroll['"],\s*handleScroll,\s*\{\s*capture:\s*true,\s*passive:\s*true\s*\}\)/
		);
		expect(componentSource).toMatch(/removeEventListener\(['"]scroll['"],\s*handleScroll/);
	});

	test('renders submitLabel on the submit button', () => {
		expect(componentSource).toMatch(/\{submitLabel\}/);
	});

	test('save() may be async and disables submit while awaiting', () => {
		expect(componentSource).toMatch(/async function save/);
		expect(componentSource).toMatch(/await onSave\(/);
		expect(componentSource).toMatch(/disabled=\{[^}]*saving[^}]*\}/);
	});

	test('keeps quote preview truncated at 80 chars', () => {
		expect(componentSource).toMatch(/quotedText\.length > 80/);
		expect(componentSource).toMatch(/quotedText\.slice\(0, 80\)/);
	});

	test('keeps cmd/ctrl+enter save shortcut', () => {
		expect(componentSource).toMatch(/e\.metaKey \|\| e\.ctrlKey/);
	});

	test('keeps dialog semantics for the composer', () => {
		expect(componentSource).toMatch(/role="dialog"/);
		expect(componentSource).toMatch(/aria-label="Add a comment"/);
	});

	test('uses Svelte 5 runes', () => {
		expect(componentSource).toMatch(/\$state|\$derived|\$props/);
	});
});
