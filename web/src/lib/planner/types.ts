import type { components } from '$lib/api/types';

export type PlanComment = components['schemas']['PlanComment'];
export type PlanCommentAnchor = components['schemas']['PlanCommentAnchor'];

/**
 * One top-level rendered block with its source-line span and its matchable
 * rendered text. `text` MUST be produced by inline-annotations' blockSearchText
 * normalization (same search space the mark applier uses) so that occurrence
 * counting agrees between capture, resolution, and mark wrapping.
 */
export type RenderedBlock = {
	lineStart: number;
	lineEnd: number;
	text: string;
	headingSlug?: string;
};

/** Captured from a user selection over the rendered document. */
export type SelectionPayload = {
	lineStart: number;
	lineEnd: number;
	quotedText: string;
	occurrence: number;
	headingSlug?: string;
	contextBefore?: string;
	contextAfter?: string;
	rect: DOMRect;
};

export type AnchorStatus = 'ok' | 'drifted' | 'orphaned';

export type AnchorResolution = {
	lineStart: number;
	lineEnd: number;
	occurrence: number;
	status: AnchorStatus;
};

/** Input to the inline mark applier. Orphaned comments never become marks. */
export type InlineMark = {
	id: string;
	quotedText: string;
	occurrence: number;
	lineStart: number;
	lineEnd: number;
	resolved: boolean;
	drifted: boolean;
};
