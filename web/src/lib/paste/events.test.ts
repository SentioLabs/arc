import { describe, it, expect } from 'vitest';
import { replayEvents, acceptedOnly } from './events';
import type { CommentEvent, ResolutionEvent } from './types';

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
});
