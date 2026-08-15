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

	it('clamps a stale occurrence (occurrence 5, 2 matches) to the last match with status drifted', () => {
		const result = resolveAnchor(blocks, anchor({ occurrence: 5 }));
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

	it('slugify matches marked heading ids ("Data Model!" → "data-model")', () => {
		expect(slugify('Data Model!')).toBe('data-model');
	});
});
