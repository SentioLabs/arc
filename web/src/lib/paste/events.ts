import type { CommentEvent, EventPlaintext, ResolutionStatus } from './types';

export type CommentState = {
	event: CommentEvent;
	status: 'open' | ResolutionStatus;
	reply?: string;
	replyAt?: string;
};

export function replayEvents(
	planAuthor: string | undefined,
	events: EventPlaintext[]
): Map<string, CommentState> {
	const sorted = [...events].sort((a, b) => {
		const c = a.created_at.localeCompare(b.created_at);
		return c !== 0 ? c : a.id.localeCompare(b.id);
	});
	const states = new Map<string, CommentState>();
	for (const e of sorted) {
		if (e.kind === 'comment') {
			states.set(e.id, { event: e, status: 'open' });
		} else if (e.kind === 'resolution') {
			const target = states.get(e.comment_id);
			if (!target) continue;
			if (planAuthor && e.author_name !== planAuthor) continue;
			target.status = e.status;
			target.reply = e.reply;
			target.replyAt = e.created_at;
		}
	}
	return states;
}

export function acceptedOnly(states: Map<string, CommentState>): CommentState[] {
	return [...states.values()].filter((s) => s.status === 'accepted');
}
