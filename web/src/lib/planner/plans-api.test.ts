import { afterEach, expect, it, vi } from 'vitest';
import { saveRevision, decideRevision, mutateComment, listLegacyPlans } from '../api/plans';
afterEach(() => vi.unstubAllGlobals());
it('saves exact bytes and displayed head using a stable caller idempotency key', async () => {
	const fetcher = vi
		.fn()
		.mockResolvedValue(new Response(JSON.stringify({ plan: {}, revision: {} })));
	vi.stubGlobal('fetch', fetcher);
	await saveRevision(
		'project',
		'plan',
		{ content: '---\nstatus: approved\n---\n', expected_revision: 2 },
		'retry-key'
	);
	const [url, init] = fetcher.mock.calls[0];
	expect(url).toBe('/api/v1/projects/project/plans/plan/revisions');
	expect(init.headers['Idempotency-Key']).toBe('retry-key');
	expect(JSON.parse(init.body)).toEqual({
		content: '---\nstatus: approved\n---\n',
		expected_revision: 2
	});
});
it('never retries decisions and preserves conflict status for the UI', async () => {
	const fetcher = vi
		.fn()
		.mockResolvedValue(
			new Response(JSON.stringify({ error: 'Stale feedback', code: 'conflict' }), { status: 409 })
		);
	vi.stubGlobal('fetch', fetcher);
	await expect(
		decideRevision('project', 'plan', 1, {
			status: 'approved',
			expected_head: 1,
			expected_review_version: 2,
			expected_feedback_version: 3
		})
	).rejects.toMatchObject({ status: 409, message: 'Stale feedback' });
	expect(fetcher).toHaveBeenCalledTimes(1);
});
it('sends the rendered comment version for deletion and bounds legacy inventory reads', async () => {
	const fetcher = vi.fn().mockImplementation(() => Promise.resolve(new Response('[]')));
	vi.stubGlobal('fetch', fetcher);
	await mutateComment('p', 'id', 1, 'c', { expected_version: 4 }, true);
	expect(JSON.parse(fetcher.mock.calls[0][1].body)).toEqual({ expected_version: 4 });
	expect(fetcher.mock.calls[0][1].method).toBe('DELETE');
	await listLegacyPlans(50);
	expect(fetcher.mock.calls[1][0]).toBe('/api/v1/plans/legacy?limit=50&offset=50');
});
