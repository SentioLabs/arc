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

/**
 * A reviewer revising their own annotation. Append-only: rather than mutating
 * the original CommentEvent blob, we emit a new EditEvent that references the
 * target comment by id and supplies new field values. Replay merges these in
 * chronological order, gated on `author_name === target.author_name` so only
 * the original author's edits take effect.
 *
 * Only `body`, `suggested_text`, and `comment_type` can be edited. Changing
 * `action` (comment vs delete) or `anchor` would change the meaning of the
 * annotation — the reviewer should delete and re-create instead.
 *
 * Field semantics:
 *   - `body` undefined  → keep current body
 *   - `body` ""         → clear body (rare but valid)
 *   - `body` non-empty  → replace body
 * Same rules for `suggested_text` and `comment_type`.
 */
export type EditEvent = {
	kind: 'edit';
	id: string;
	comment_id: string;
	author_name: string;
	body?: string;
	suggested_text?: string;
	comment_type?: CommentType;
	created_at: string;
};

export type PlanEditEvent = {
	kind: 'plan_edit';
	id: string;
	edit_summary?: string;
	created_at: string;
};

/**
 * The original commenter retracting their own annotation. Replay marks the
 * target comment with status='retracted'; UI hides retracted comments and
 * their inline marks. The encrypted event stays in the log so the action is
 * auditable, but `arc share comments` filters retracted entries out of the
 * default output (LLM consumers shouldn't act on retracted material).
 *
 * Authorization is replay-time: only an event whose `author_name` matches
 * the target comment's `author_name` takes effect. The plan author canNOT
 * retract someone else's comment — they must use Reject (with a reply that
 * preserves rationale in the audit trail).
 */
export type RetractionEvent = {
	kind: 'retraction';
	id: string;
	comment_id: string;
	author_name: string;
	created_at: string;
};

export type EventPlaintext =
	| CommentEvent
	| ResolutionEvent
	| EditEvent
	| RetractionEvent
	| PlanEditEvent;
