export type PasteShareResponse = {
	id: string;
	plan_blob: string;
	plan_iv: string;
	schema_ver: number;
	created_at: string;
	updated_at: string;
	expires_at?: string;
};

export type PasteEventResponse = {
	id: string;
	share_id: string;
	blob: string;
	iv: string;
	created_at: string;
};

export type GetPasteResponse = PasteShareResponse & { events: PasteEventResponse[] };

export type CreatePasteRequest = {
	plan_blob: string;
	plan_iv: string;
	schema_ver: number;
	expires_at?: string;
};

export type CreatePasteResponse = { id: string; edit_token: string };

export type AppendEventRequest = { blob: string; iv: string };

export type PlanPlaintext = {
	version: 1;
	markdown: string;
	title?: string;
	author_name?: string;
	created_at: string;
};

export type CommentType = 'comment' | 'praise' | 'issue' | 'suggestion' | 'question' | 'nit';
export type Severity = 'important' | 'nit';
export type ResolutionStatus = 'accepted' | 'rejected' | 'resolved' | 'reopened';

export type Anchor = {
	line_start: number;
	line_end: number;
	char_start?: number;
	char_end?: number;
	quoted_text: string;
	context_before?: string;
	context_after?: string;
	heading_slug?: string;
};

/**
 * The reviewer's intent. Plannotator parity: COMMENT vs DELETION are the
 * primary actions. `comment_type` (praise/issue/etc.) is a secondary label.
 * Body is required for action='comment' but optional for 'delete' — the
 * strikethrough IS the action.
 */
export type AnnotationAction = 'comment' | 'delete';

export type CommentEvent = {
	kind: 'comment';
	id: string;
	author_name: string;
	/** Primary intent. Defaults to 'comment' for back-compat with v1 events. */
	action?: AnnotationAction;
	comment_type: CommentType;
	severity?: Severity;
	/** Required for action='comment'; may be empty for action='delete'. */
	body: string;
	suggested_text?: string;
	parent_id?: string;
	anchor: Anchor;
	created_at: string;
};

export type ResolutionEvent = {
	kind: 'resolution';
	id: string;
	comment_id: string;
	status: ResolutionStatus;
	reply?: string;
	author_name: string;
	created_at: string;
};

export type PlanEditEvent = {
	kind: 'plan_edit';
	id: string;
	edit_summary?: string;
	created_at: string;
};

export type EventPlaintext = CommentEvent | ResolutionEvent | PlanEditEvent;
