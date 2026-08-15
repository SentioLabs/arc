import { describe, expect, it } from 'vitest';
import { resolveAnchor, slugify } from './anchor';
import type { PlanCommentAnchor, RenderedBlock } from './types';

const blocks: RenderedBlock[] = [
	{ lineStart: 1, lineEnd: 1, text: 'Data Model', headingSlug: 'data-model' },
	{ lineStart: 3, lineEnd: 5, text: 'run task build to compile everything now' },
	{ lineStart: 7, lineEnd: 7, text: 'Testing', headingSlug: 'testing' },
	{ lineStart: 9, lineEnd: 12, text: 'run task build again and again for tests' }
];

function anchor(partial: Partial<PlanCommentAnchor>): PlanCommentAnchor {
	return { line_start: 3, line_end: 5, quoted_text: 'run task build', occurrence: 0, ...partial };
}

describe('resolveAnchor', () => {
	it('resolves an exact unique quote as ok', () => {
		const result = resolveAnchor(
			blocks,
			anchor({ line_start: 7, line_end: 7, quoted_text: 'Testing', occurrence: 0 })
		);
		expect(result).toEqual({ lineStart: 7, lineEnd: 7, occurrence: 0, status: 'ok' });
	});

	it('respects occurrence for repeated quotes (occurrence 1 → second block, status ok when lines match)', () => {
		const result = resolveAnchor(blocks, anchor({ line_start: 9, line_end: 12, occurrence: 1 }));
		expect(result).toEqual({ lineStart: 9, lineEnd: 12, occurrence: 1, status: 'ok' });
	});

	it('reports drifted when the quote moved to different lines but occurrence still matches', () => {
		// Stored lines (99, 99) no longer match where occurrence 0 actually resolves
		// (block at lines 3-5) — same occurrence index, different lines => drifted.
		const result = resolveAnchor(blocks, anchor({ line_start: 99, line_end: 99, occurrence: 0 }));
		expect(result).toEqual({ lineStart: 3, lineEnd: 5, occurrence: 0, status: 'drifted' });
	});

	it('repairs a stale occurrence (occurrence 5, 2 matches) to the match nearest the stored line', () => {
		// Stored line_start 3 sits on the first match's block (lines 3-5), so the
		// nearest-to-stored rule keeps the comment where it was rather than
		// clamping to the last match.
		const result = resolveAnchor(blocks, anchor({ occurrence: 5 }));
		expect(result).toEqual({ lineStart: 3, lineEnd: 5, occurrence: 0, status: 'drifted' });
	});

	it('repairs a stale occurrence toward the later match when the stored line is nearer to it', () => {
		const result = resolveAnchor(blocks, anchor({ line_start: 10, line_end: 12, occurrence: 5 }));
		expect(result).toEqual({ lineStart: 9, lineEnd: 12, occurrence: 1, status: 'drifted' });
	});

	it('uses context_before/context_after to disambiguate when occurrence is stale', () => {
		// context_before "Model\n" only immediately precedes the first occurrence
		// (lines 3-5). Blind clamping to the last match would pick lines 9-12,
		// so this proves context wins over the naive clamp.
		const result = resolveAnchor(blocks, anchor({ occurrence: 9, context_before: 'Model\n' }));
		expect(result).toEqual({ lineStart: 3, lineEnd: 5, occurrence: 0, status: 'drifted' });
	});

	it('prefers the match under the stored heading_slug when context is absent and occurrence is stale', () => {
		// heading_slug "data-model" is nearest to the first occurrence (lines 3-5)
		// only. Blind clamping to the last match would pick lines 9-12, so this
		// proves heading disambiguation wins over the naive clamp.
		const result = resolveAnchor(blocks, anchor({ occurrence: 9, heading_slug: 'data-model' }));
		expect(result).toEqual({ lineStart: 3, lineEnd: 5, occurrence: 0, status: 'drifted' });
	});

	it('returns orphaned with the stored lines when the quote is gone', () => {
		const result = resolveAnchor(blocks, anchor({ quoted_text: 'no such text' }));
		expect(result).toEqual({ lineStart: 3, lineEnd: 5, occurrence: 0, status: 'orphaned' });
	});

	it('returns orphaned for an empty quoted_text (never a false ok)', () => {
		const result = resolveAnchor(blocks, anchor({ quoted_text: '' }));
		expect(result).toEqual({ lineStart: 3, lineEnd: 5, occurrence: 0, status: 'orphaned' });
		expect(result.status).not.toBe('ok');
	});

	it('resolves a quote spanning two blocks (needle contains the \\n join)', () => {
		const result = resolveAnchor(
			blocks,
			anchor({
				line_start: 7,
				line_end: 12,
				quoted_text: 'Testing\nrun task build',
				occurrence: 0
			})
		);
		expect(result).toEqual({ lineStart: 7, lineEnd: 12, occurrence: 0, status: 'ok' });
	});

	it('picks the context-qualified duplicate nearest the stored line, not the first one', () => {
		// Templated content: both paragraphs carry the same quote AND the same
		// context_before, so context alone cannot disambiguate. The stored line
		// (3) points at the second copy — document order would return the first.
		const duplicated: RenderedBlock[] = [
			{ lineStart: 1, lineEnd: 1, text: 'AAAA run task build XX' },
			{ lineStart: 3, lineEnd: 3, text: 'AAAA run task build' }
		];
		const result = resolveAnchor(
			duplicated,
			anchor({ line_start: 3, line_end: 3, occurrence: 9, context_before: 'AAAA ' })
		);
		expect(result).toEqual({ lineStart: 3, lineEnd: 3, occurrence: 1, status: 'drifted' });
	});

	it('picks the match nearest the stored line in the no-signal fallback', () => {
		const repeated: RenderedBlock[] = [
			{ lineStart: 1, lineEnd: 1, text: 'run task build' },
			{ lineStart: 11, lineEnd: 11, text: 'run task build' },
			{ lineStart: 21, lineEnd: 21, text: 'run task build' }
		];
		const result = resolveAnchor(repeated, anchor({ line_start: 11, line_end: 11, occurrence: 9 }));
		expect(result).toEqual({ lineStart: 11, lineEnd: 11, occurrence: 1, status: 'drifted' });
	});

	it('breaks a nearest-distance tie toward the earlier match', () => {
		const equidistant: RenderedBlock[] = [
			{ lineStart: 1, lineEnd: 1, text: 'run task build' },
			{ lineStart: 5, lineEnd: 5, text: 'run task build' }
		];
		const result = resolveAnchor(
			equidistant,
			anchor({ line_start: 3, line_end: 3, occurrence: 9 })
		);
		expect(result).toEqual({ lineStart: 1, lineEnd: 1, occurrence: 0, status: 'drifted' });
	});

	it('resolves a cross-paragraph quote whose \\n\\n does not match the single-\\n index', () => {
		// selection.toString() emits a blank line between paragraphs; the doc
		// index joins blocks with one \n, so the exact search finds nothing and
		// the whitespace-normalized tier has to recover it (never as ok).
		const paragraphs: RenderedBlock[] = [
			{ lineStart: 1, lineEnd: 1, text: 'First paragraph.' },
			{ lineStart: 3, lineEnd: 3, text: 'Second one.' }
		];
		const result = resolveAnchor(
			paragraphs,
			anchor({
				line_start: 1,
				line_end: 3,
				quoted_text: 'First paragraph.\n\nSecond one.',
				occurrence: 0
			})
		);
		expect(result).toEqual({ lineStart: 1, lineEnd: 3, occurrence: 0, status: 'drifted' });
	});

	it('slugify matches marked heading ids ("Data Model!" → "data-model")', () => {
		expect(slugify('Data Model!')).toBe('data-model');
	});
});
