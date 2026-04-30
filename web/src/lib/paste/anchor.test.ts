import { describe, it, expect } from 'vitest';
import { resolveAnchor } from './anchor';

describe('resolveAnchor', () => {
	const plan = '# Title\n\nFirst paragraph.\nSecond paragraph.\n## Sub\nThird.\n';

	it('returns ok when quoted_text still at original lines', () => {
		const r = resolveAnchor(plan, { line_start: 3, line_end: 3, quoted_text: 'First paragraph.' });
		expect(r.status).toBe('ok');
	});

	it('returns drifted when found via heading slug', () => {
		const edited = 'PRELUDE\n# Title\n\nMore content.\nFirst paragraph.\n## Sub\nThird.\n';
		const r = resolveAnchor(edited, {
			line_start: 3,
			line_end: 3,
			quoted_text: 'First paragraph.',
			heading_slug: 'title'
		});
		expect(r.status).toBe('drifted');
	});

	it('returns orphaned when text is gone', () => {
		const edited = '# Title\n\nDifferent stuff.\n';
		const r = resolveAnchor(edited, {
			line_start: 3,
			line_end: 3,
			quoted_text: 'First paragraph.',
			heading_slug: 'title'
		});
		expect(r.status).toBe('orphaned');
	});
});
