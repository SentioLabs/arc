import { describe, it, expect } from 'vitest';
import { replayEvents, acceptedOnly } from './events';
import type { CommentEvent, EditEvent, ResolutionEvent } from './types';

const c: CommentEvent = {
	kind: 'comment',
	id: 'c1',
	author_name: 'Alice',
	comment_type: 'comment',
	body: 'looks good',
	anchor: { line_start: 1, line_end: 1, quoted_text: 'x' },
	created_at: '2026-04-29T00:00:00Z'
};
const accept: ResolutionEvent = {
	kind: 'resolution',
	id: 'r1',
	comment_id: 'c1',
	status: 'accepted',
	author_name: 'Ben',
	created_at: '2026-04-29T00:01:00Z'
};

describe('replayEvents', () => {
	it('marks comment accepted when resolver is plan author', () => {
		const states = replayEvents('Ben', [c, accept]);
		expect(states.get('c1')?.status).toBe('accepted');
	});

	it('ignores resolution from non-author', () => {
		const states = replayEvents('Ben', [c, { ...accept, author_name: 'Mallory' }]);
		expect(states.get('c1')?.status).toBe('open');
	});

	it('acceptedOnly filters non-accepted', () => {
		const states = replayEvents('Ben', [c, accept]);
		expect(acceptedOnly(states).length).toBe(1);
	});

	it('replays in created_at order', () => {
		const reject: ResolutionEvent = {
			...accept,
			id: 'r2',
			status: 'rejected',
			created_at: '2026-04-29T00:02:00Z'
		};
		const states = replayEvents('Ben', [c, accept, reject]);
		expect(states.get('c1')?.status).toBe('rejected');
	});

	describe('edit events', () => {
		const baseEdit: EditEvent = {
			kind: 'edit',
			id: 'e1',
			comment_id: 'c1',
			author_name: 'Alice',
			body: 'expanded reasoning: the issue is X because Y',
			created_at: '2026-04-29T00:05:00Z'
		};

		it("applies edits to the original author's comment", () => {
			const states = replayEvents('Ben', [c, baseEdit]);
			const s = states.get('c1');
			expect(s?.event.body).toBe('expanded reasoning: the issue is X because Y');
			expect(s?.editedAt).toBe('2026-04-29T00:05:00Z');
		});

		it('discards edits forged by someone else', () => {
			const forged: EditEvent = { ...baseEdit, author_name: 'Mallory' };
			const states = replayEvents('Ben', [c, forged]);
			const s = states.get('c1');
			expect(s?.event.body).toBe('looks good'); // unchanged
			expect(s?.editedAt).toBeUndefined();
		});

		it('applies multiple edits in chronological order', () => {
			const second: EditEvent = {
				...baseEdit,
				id: 'e2',
				body: 'final wording',
				created_at: '2026-04-29T00:10:00Z'
			};
			// Pass them out of order to confirm sorting is what matters.
			const states = replayEvents('Ben', [c, second, baseEdit]);
			expect(states.get('c1')?.event.body).toBe('final wording');
			expect(states.get('c1')?.editedAt).toBe('2026-04-29T00:10:00Z');
		});

		it('only updates fields explicitly present in the edit', () => {
			const withSuggestion: CommentEvent = {
				...c,
				suggested_text: 'original replacement'
			};
			// Edit changes body but not suggested_text — original suggestion must persist.
			const bodyOnly: EditEvent = {
				...baseEdit,
				body: 'new body',
				suggested_text: undefined
			};
			const states = replayEvents('Ben', [withSuggestion, bodyOnly]);
			expect(states.get('c1')?.event.body).toBe('new body');
			expect(states.get('c1')?.event.suggested_text).toBe('original replacement');
		});

		it('treats empty string as an explicit clear', () => {
			const withSuggestion: CommentEvent = {
				...c,
				suggested_text: 'replacement'
			};
			const clearSuggestion: EditEvent = {
				...baseEdit,
				body: undefined,
				suggested_text: ''
			};
			const states = replayEvents('Ben', [withSuggestion, clearSuggestion]);
			expect(states.get('c1')?.event.suggested_text).toBe('');
		});

		it('drops edits for nonexistent comments without throwing', () => {
			const orphan: EditEvent = { ...baseEdit, comment_id: 'does-not-exist' };
			const states = replayEvents('Ben', [c, orphan]);
			expect(states.size).toBe(1); // c1 still present, orphan ignored
		});

		it('preserves resolution + edit interleaving', () => {
			// Comment → accepted → edited. Status must remain 'accepted' but
			// the body must reflect the edit (the author refining their note
			// after acceptance).
			const states = replayEvents('Ben', [c, accept, baseEdit]);
			const s = states.get('c1');
			expect(s?.status).toBe('accepted');
			expect(s?.event.body).toBe('expanded reasoning: the issue is X because Y');
		});
	});
});
