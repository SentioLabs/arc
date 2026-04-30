import type { CommentEvent, EventPlaintext, ResolutionStatus } from './types';

export type CommentState = {
	event: CommentEvent;
	status: 'open' | ResolutionStatus;
	reply?: string;
	replyAt?: string;
	/**
	 * Timestamp of the most recent successfully-applied EditEvent for this
	 * comment. Undefined if the comment has never been edited. The UI uses
	 * this to render an "edited" indicator.
	 */
	editedAt?: string;
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
		} else if (e.kind === 'edit') {
			const target = states.get(e.comment_id);
			if (!target) continue;
			// Edits are accepted from two roles:
			//   1. The original author of the comment (refining their own wording).
			//   2. The plan author (sharpening reviewer feedback into something
			//      LLM-consumable without waiting on the reviewer).
			// The displayed `comment.author_name` does NOT change either way —
			// the edit event in the log records who actually edited.
			//
			// `planAuthor` must be non-empty before granting plan-owner edit
			// rights; otherwise empty strings would all match each other.
			const isOriginalAuthor = e.author_name === target.event.author_name;
			const isPlanAuthor = !!planAuthor && e.author_name === planAuthor;
			if (!isOriginalAuthor && !isPlanAuthor) continue;
			// Build a new event object with the supplied fields applied. We
			// replace `target.event` rather than mutate so consumers that hold
			// a stale reference don't see partial state during the merge.
			target.event = {
				...target.event,
				body: e.body !== undefined ? e.body : target.event.body,
				suggested_text:
					e.suggested_text !== undefined ? e.suggested_text : target.event.suggested_text,
				comment_type: e.comment_type !== undefined ? e.comment_type : target.event.comment_type
			};
			target.editedAt = e.created_at;
		}
	}
	return states;
}

export function acceptedOnly(states: Map<string, CommentState>): CommentState[] {
	return [...states.values()].filter((s) => s.status === 'accepted');
}
