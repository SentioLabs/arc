import type { components } from '../api/types';
type Schema = components['schemas'];

export function reviewContext(
	plan: Pick<Schema['Plan'], 'head_revision' | 'feedback_version'>,
	revision: Pick<Schema['PlanRevision'], 'review_version'>
) {
	return {
		expected_head: plan.head_revision,
		expected_review_version: revision.review_version,
		expected_feedback_version: plan.feedback_version
	};
}

export function feedbackDisposition(
	comment: Schema['PlanComment'],
	dispositions: Schema['PlanFeedbackDisposition'][],
	revision: number
) {
	return dispositions.findLast(
		(d) =>
			d.comment_id === comment.id &&
			d.comment_version === comment.version &&
			d.target_revision <= revision &&
			(d.disposition === 'addressed' || d.target_revision === revision)
	);
}
