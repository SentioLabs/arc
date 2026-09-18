import { describe, it, expect } from 'vitest';
import { feedbackDisposition, reviewContext } from './review';
import type { components } from '$lib/api/types';
type Comment = components['schemas']['PlanComment'];
type Disposition = components['schemas']['PlanFeedbackDisposition'];
const comment: Comment = {
	id: 'c',
	plan_id: 'p',
	revision: 1,
	version: 2,
	content: 'Fix',
	created_at: 'now'
};
const addressed: Disposition = {
	id: 'd',
	plan_id: 'p',
	target_revision: 1,
	comment_id: 'c',
	comment_version: 2,
	disposition: 'addressed',
	reason: 'Fixed',
	created_at: 'now'
};
describe('revision review obligations', () => {
	it('carries addressed reasons forward but never across edited/reopened/deleted versions', () => {
		expect(feedbackDisposition(comment, [addressed], 2)).toEqual(addressed);
		expect(
			feedbackDisposition({ ...comment, version: 3, deleted_at: 'now' }, [addressed], 2)
		).toBeUndefined();
	});
	it('requires reconsidering deferral at the next revision and retains prior dispositions', () => {
		const deferred = { ...addressed, disposition: 'deferred' };
		expect(feedbackDisposition(comment, [deferred], 1)).toEqual(deferred);
		expect(feedbackDisposition(comment, [deferred], 2)).toBeUndefined();
		expect(feedbackDisposition(comment, [addressed], 0)).toBeUndefined();
	});
	it('captures displayed versions by value', () => {
		const plan = { head_revision: 2, feedback_version: 3 };
		const revision = { review_version: 4 };
		const captured = reviewContext(plan, revision);
		plan.head_revision = 5;
		revision.review_version = 6;
		expect(captured).toEqual({
			expected_head: 2,
			expected_feedback_version: 3,
			expected_review_version: 4
		});
	});
});
