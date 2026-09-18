// @vitest-environment jsdom
// The removed filesystem FileTree has no supported UI. Navigation is now through
// durable project/plan/revision links; exercise those rendered links instead.
import { afterEach, expect, test, vi } from 'vitest';
import { mount, unmount, flushSync } from 'svelte';
import DesignContext from './DesignContext.svelte';
let component: ReturnType<typeof mount>;
afterEach(async () => {
	if (component) await unmount(component);
	document.body.innerHTML = '';
	vi.unstubAllGlobals();
});
test('renders explicit pinned revision navigation even with no inherited design', async () => {
	vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('null')));
	component = mount(DesignContext, {
		target: document.body,
		props: {
			projectId: 'project-a',
			issueId: 'TASK-1',
			explicitPin: { plan_id: 'plan-a', revision: 3 }
		}
	});
	flushSync();
	await vi.waitFor(() => expect(document.body.textContent).toContain('No governing plan'));
	const link = document.querySelector('a');
	expect(link?.textContent).toBe('Pinned revision 3');
	expect(link?.getAttribute('href')).toBe('/project-a/plans/plan-a/3');
});
test('failed governance shows actionable error and retry recovers', async () => {
	const fetcher = vi
		.fn()
		.mockResolvedValueOnce(
			new Response(
				JSON.stringify({ error: 'Conflicting governing designs', code: 'ambiguous_governance' }),
				{ status: 409 }
			)
		)
		.mockResolvedValueOnce(new Response('null'));
	vi.stubGlobal('fetch', fetcher);
	component = mount(DesignContext, {
		target: document.body,
		props: { projectId: 'project-a', issueId: 'TASK-1' }
	});
	flushSync();
	await vi.waitFor(() =>
		expect(document.querySelector('[role="alert"]')?.textContent).toContain(
			'Conflicting governing designs'
		)
	);
	expect(document.body.textContent).toContain('Reconcile the parent hierarchy');
	document.querySelector('button')?.click();
	await vi.waitFor(() => expect(document.body.textContent).toContain('No governing plan'));
	expect(document.querySelector('[role="alert"]')).toBeNull();
	expect(fetcher).toHaveBeenCalledTimes(2);
});
