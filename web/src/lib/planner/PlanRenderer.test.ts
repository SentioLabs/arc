// @vitest-environment jsdom
import { describe, expect, test } from 'vitest';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, resolve } from 'node:path';
import { lexMarkdown, renderMarkdown } from '../markdown';

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);
const componentSource = readFileSync(resolve(__dirname, 'PlanRenderer.svelte'), 'utf-8');

const SAMPLE = [
	'# Title',
	'',
	'Intro paragraph with [a ref][r].',
	'',
	'<!-- a stripped comment -->',
	'',
	'```sql',
	'SELECT 1;',
	'',
	'-- not a heading',
	'SELECT 2;',
	'```',
	'',
	'- one',
	'- two',
	'',
	'[r]: https://example.com',
	''
].join('\n');

describe('lexMarkdown', () => {
	test('keeps a fenced code block with internal blank lines as a single token', async () => {
		const tokens = await lexMarkdown(SAMPLE);
		const code = tokens.filter((t) => t.type === 'code');
		expect(code).toHaveLength(1);
		expect(code[0].raw).toContain('SELECT 2;');
	});

	test('emits token.raw values as literal in-order substrings of the source', async () => {
		const tokens = await lexMarkdown(SAMPLE);
		let cursor = 0;
		for (const t of tokens) {
			const idx = SAMPLE.indexOf(t.raw, cursor);
			expect(idx).toBeGreaterThanOrEqual(0);
			cursor = idx + t.raw.length;
		}
	});

	test('tokenizes with the configured instance, so gfm lists survive', async () => {
		const tokens = await lexMarkdown(SAMPLE);
		expect(tokens.some((t) => t.type === 'list')).toBe(true);
	});
});

describe('lex/render alignment invariant', () => {
	// PlanRenderer maps top-level tokens onto top-level rendered elements
	// positionally. That only holds if `space`, `def` and HTML-comment tokens
	// render nothing — this pins that assumption to the real pipeline.
	test('space, def and HTML-comment tokens produce no top-level element', async () => {
		const tokens = await lexMarkdown(SAMPLE);
		const rendering = tokens.filter(
			(t) =>
				t.type !== 'space' &&
				t.type !== 'def' &&
				!(t.type === 'html' && t.raw.trim().startsWith('<!--') && t.raw.trim().endsWith('-->'))
		);
		expect(tokens.length).toBeGreaterThan(rendering.length);

		const html = await renderMarkdown(SAMPLE);
		const doc = new DOMParser().parseFromString(`<div id="r">${html}</div>`, 'text/html');
		const root = doc.getElementById('r');
		expect(root?.children.length).toBe(rendering.length);
	});
});

describe('PlanRenderer component', () => {
	test('exposes the shared props contract consumed by the review page', () => {
		expect(componentSource).toMatch(/markdown/);
		expect(componentSource).toMatch(/marks = \[\]/);
		expect(componentSource).toMatch(/activeMarkId/);
		expect(componentSource).toMatch(/onSelection/);
		expect(componentSource).toMatch(/onMarkClick/);
		expect(componentSource).toMatch(/onBlocks/);
		expect(componentSource).toMatch(/onBlocks\?:\s*\(blocks: RenderedBlock\[\]\) => void/);
		expect(componentSource).toMatch(/onSelection\?:\s*\(sel: SelectionPayload \| null\) => void/);
	});

	test('imports the shared planner modules without .ts extensions', () => {
		expect(componentSource).toMatch(
			/import \{[^}]*lexMarkdown[^}]*\} from ['"]\$lib\/markdown['"]/
		);
		expect(componentSource).toMatch(
			/import \{[^}]*renderMarkdown[^}]*\} from ['"]\$lib\/markdown['"]/
		);
		expect(componentSource).toMatch(/CONTEXT_CHARS[^}]*\} from ['"]\.\/anchor['"]/);
		expect(componentSource).toMatch(/slugify/);
		expect(componentSource).toMatch(
			/import \{[^}]*applyInlineAnnotations[^}]*\} from ['"]\.\/inline-annotations['"]/
		);
		expect(componentSource).toMatch(
			/import \{[^}]*blockSearchText[^}]*\} from ['"]\.\/inline-annotations['"]/
		);
		expect(componentSource).not.toMatch(/from ['"]\.\/anchor\.ts['"]/);
		expect(componentSource).not.toMatch(/from ['"]\.\/inline-annotations\.ts['"]/);
	});

	test('has no dead references to the removed $lib/paste module', () => {
		expect(componentSource).not.toMatch(/\$lib\/paste/);
	});

	test('normalizes CRLF before lexing and rendering', () => {
		expect(componentSource).toMatch(/replace\(\/\\r\\n\?\/g, '\\n'\)/);
	});

	test('tags top-level blocks with a source line SPAN', () => {
		expect(componentSource).toMatch(/setAttribute\('data-source-line', String\(/);
		expect(componentSource).toMatch(/setAttribute\('data-source-line-end', String\(/);
	});

	test('skips tokens that render no top-level element', () => {
		expect(componentSource).toMatch(/'space'/);
		expect(componentSource).toMatch(/'def'/);
		expect(componentSource).toMatch(/startsWith\('<!--'\)/);
		expect(componentSource).toMatch(/endsWith\('-->'\)/);
	});

	test('hardens alignment by tagging only the overlapping prefix', () => {
		expect(componentSource).toMatch(/Math\.min\(children\.length, spans\.length\)/);
	});

	test('publishes RenderedBlock[] using blockSearchText and heading slugs', () => {
		expect(componentSource).toMatch(/function collectBlocks/);
		expect(componentSource).toMatch(/text: blockSearchText\(/);
		expect(componentSource).toMatch(/headingSlug: isHeading \? slugify\(/);
		expect(componentSource).toMatch(/onBlocks\?\.\(collectBlocks\(/);
		expect(componentSource).not.toMatch(/text: [^\n]*textContent/);
	});

	test('finds a heading when the block itself IS the heading', () => {
		expect(componentSource).toMatch(/matches\('h1, h2, h3, h4, h5, h6'\)/);
	});

	test('captures selections via selectionchange so keyboard/touch work', () => {
		expect(componentSource).toMatch(/addEventListener\('selectionchange'/);
		expect(componentSource).toMatch(/removeEventListener\('selectionchange'/);
		expect(componentSource).toMatch(/150/);
		expect(componentSource).not.toMatch(/onmouseup/);
		expect(componentSource).not.toMatch(/handleMouseUp/);
	});

	test('routes mark clicks through pointerup, ignoring drag-ends on a mark', () => {
		expect(componentSource).toMatch(/onpointerup=\{/);
		expect(componentSource).toMatch(/closest\('mark\[data-anno-id\]'\)/);
		expect(componentSource).toMatch(/onMarkClick\?\.\(/);
	});

	test('counts occurrences over the selection-semantics prefix range', () => {
		expect(componentSource).toMatch(/function countOccurrences/);
		expect(componentSource).toMatch(/prefix\.selectNodeContents\(container\)/);
		expect(componentSource).toMatch(/prefix\.setEnd\(range\.startContainer, range\.startOffset\)/);
		expect(componentSource).toMatch(/countOccurrences\(prefixText, quotedText\)/);
	});

	test('captures CONTEXT_CHARS-sized contexts from the same prefix/suffix space', () => {
		expect(componentSource).toMatch(/prefixText\.slice\(-CONTEXT_CHARS\)/);
		expect(componentSource).toMatch(/suffix\.setStart\(range\.endContainer, range\.endOffset\)/);
		expect(componentSource).toMatch(/slice\(0, CONTEXT_CHARS\)/);
		expect(componentSource).not.toMatch(/\b40\b/);
	});

	test('keeps the document surface template shape', () => {
		expect(componentSource).toMatch(/<article bind:this=\{container\} class="doc"/);
		expect(componentSource).toMatch(/\{@html html\}/);
	});
});
