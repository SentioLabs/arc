import { test, expect } from '@playwright/test';
import { createTestWorkspace, uniqueName } from './fixtures';
const base = 'http://localhost:7433/api/v1';
async function request(path: string, body?: unknown, method = 'POST') {
	const response = await fetch(
		base + path,
		body === undefined
			? undefined
			: {
					method,
					headers: { 'Content-Type': 'application/json', 'Idempotency-Key': uniqueName('plan') },
					body: JSON.stringify(body)
				}
	);
	expect(response.ok, await response.clone().text()).toBeTruthy();
	return response.json();
}
async function seed() {
	const project = await createTestWorkspace();
	const { plan } = await request(`/projects/${project.id}/plans`, {
		title: 'Server title',
		content: '---\nstatus: approved\n---\n# Original bytes\n\nOriginal context.'
	});
	return {
		project,
		plan,
		path: `/projects/${project.id}/plans/${plan.id}`,
		url: `/${project.id}/plans/${plan.id}`
	};
}
test('sidebar, exact revision, explicit save, immutable history and archive', async ({ page }) => {
	const { project, path, url } = await seed();
	await page.goto(`/${project.id}`);
	await page.getByRole('link', { name: 'Plans', exact: true }).click();
	await expect(page).toHaveURL(new RegExp(`/${project.id}/plans$`));
	await page.getByRole('link', { name: 'Server title' }).click();
	await expect(page).toHaveURL(new RegExp(`${url}/1$`));
	await expect(page.locator('article.doc')).toContainText('Original bytes');
	await expect(page.getByTestId('revision-metadata')).toContainText('draft');
	if (process.env.ARC_UI_EVIDENCE_DIR)
		await page.screenshot({
			path: `${process.env.ARC_UI_EVIDENCE_DIR}/revision-review.png`,
			fullPage: true
		});
	await page.getByRole('button', { name: 'Submit for review' }).click();
	await page.getByRole('button', { name: 'Approve', exact: true }).click();
	await expect(page.getByTestId('revision-metadata')).toContainText('approved');
	await page.getByRole('button', { name: 'Edit', exact: true }).click();
	await page.getByLabel('Plan content').fill('# Second bytes');
	expect((await request(path)).head_revision).toBe(1);
	await page.getByRole('button', { name: 'Save', exact: true }).click();
	await expect(page).toHaveURL(new RegExp(`${url}/2$`));
	await expect(page.locator('article.doc')).toContainText('Second bytes');
	await expect(page.getByTestId('revision-metadata')).toContainText('draft');
	await page.getByRole('link', { name: 'Revision 1', exact: true }).click();
	await expect(page.locator('article.doc')).toContainText('Original bytes');
	await expect(page.getByTestId('revision-metadata')).toContainText('approved');
	await expect(page.getByText(/Superseded/)).toBeVisible();
	await page.getByRole('button', { name: 'Archive plan' }).click();
	await expect(page.getByTestId('revision-metadata')).toContainText('archived');
	await page.reload();
	await expect(page.locator('article.doc')).toContainText('Original bytes');
	await page.getByRole('button', { name: 'Restore plan' }).click();
	await expect(page.getByTestId('revision-metadata')).toContainText('approved');
});
test('head conflict preserves the draft and stale approval never follows a new head', async ({
	page,
	browser
}) => {
	const { path, url } = await seed();
	await page.goto(`${url}/1`);
	await expect(page.locator('article.doc')).toContainText('Original bytes');
	await page.getByRole('button', { name: 'Submit for review' }).click();
	const other = await browser.newPage();
	await other.goto(`${url}/1`);
	await expect(other.locator('article.doc')).toContainText('Original bytes');
	await other.getByRole('button', { name: 'Edit', exact: true }).click();
	await other.getByLabel('Plan content').fill('# Retained local draft');
	await request(`${path}/revisions`, { content: '# Concurrent head', expected_revision: 1 });
	await other.getByRole('button', { name: 'Save', exact: true }).click();
	await expect(other.getByRole('alert')).toContainText(/conflict|head|revision/i);
	await expect(other.getByLabel('Plan content')).toHaveValue('# Retained local draft');
	await page.getByRole('button', { name: 'Approve', exact: true }).click();
	await expect(page.getByRole('alert')).toBeVisible();
	await expect(page).toHaveURL(new RegExp(`${url}/1$`));
	await expect(page.locator('article.doc')).toContainText('Original bytes');
	expect((await request(`${path}/revisions/2`)).review_status).toBe('draft');
	await other.close();
});
test('missing legacy plan explains explicit migration', async ({ page }) => {
	await page.goto('/planner/plan.missing');
	await expect(page.getByText(/missing or unimported/i)).toBeVisible();
	await expect(page.getByText(/Use arc server plans migrate/)).toBeVisible();
});

test('earlier anchors remain original, deferral expires, and stale comment drafts survive', async ({
	page
}) => {
	const { path, url } = await seed();
	const comment = await request(`${path}/revisions/1/comments`, {
		content: 'Explain original context',
		anchor: { line_start: 6, line_end: 6, quoted_text: 'Original context.', occurrence: 0 }
	});
	await page.goto(`${url}/1`);
	await expect(page.locator('article.doc')).toContainText('Original context.');
	await page.locator('[data-comment-id]').getByRole('button', { name: 'Edit comment' }).click();
	await page.locator('[data-comment-id] textarea').fill('My retained comment draft');
	await request(
		`${path}/revisions/1/comments/${comment.id}`,
		{
			content: 'Concurrent edit',
			expected_version: comment.version,
			anchor: { line_start: 6, line_end: 6, quoted_text: 'Concurrent remote anchor', occurrence: 0 }
		},
		'PATCH'
	);
	await page
		.locator('[data-comment-id]')
		.getByRole('button', { name: 'Save', exact: true })
		.click();
	await expect(page.getByRole('alert')).toBeVisible();
	await expect(page.locator('[data-comment-id] textarea')).toHaveValue('My retained comment draft');
	await request(`${path}/revisions`, {
		content: '# Second content\n\nNo old phrase here.',
		expected_revision: 1
	});
	await page.goto(`${url}/2`);
	await expect(page.locator('article.doc')).toContainText('Second content');
	await expect(page.locator('article.doc mark')).toHaveCount(0);
	await expect(page.locator('[data-comment-id]')).toHaveCount(0);
	const feedback = page.locator(`[data-feedback-id="${comment.id}"]`);
	await expect(feedback).toContainText('Concurrent remote anchor');
	await expect(feedback.getByRole('link', { name: 'Original revision 1' })).toHaveAttribute(
		'href',
		`${url}/1`
	);
	await page.getByRole('button', { name: 'Submit for review' }).click();
	await page.getByRole('button', { name: 'Approve', exact: true }).click();
	await expect(page.getByRole('alert')).toContainText(/feedback|disposition/i);
	await feedback.getByRole('button', { name: 'Assess feedback' }).click();
	await page.getByLabel('Disposition', { exact: true }).selectOption('deferred');
	await page.getByLabel('Disposition reason').fill('Defer pending design follow-up');
	await page.getByRole('button', { name: 'Record disposition' }).click();
	await expect(feedback).toContainText('deferred: Defer pending design follow-up');
	await page.getByRole('button', { name: 'Approve', exact: true }).click();
	await expect(page.getByTestId('revision-metadata')).toContainText('approved');
	await request(`${path}/revisions`, { content: '# Third content', expected_revision: 2 });
	await page.goto(`${url}/3`);
	await expect(page.locator('article.doc')).toContainText('Third content');
	await expect(feedback).toContainText('Outstanding');
	await page.getByText('Disposition history (1)', { exact: true }).click();
	await expect(page.getByText(/revision 2: deferred/)).toBeVisible();
});

test('concurrent feedback invalidates the displayed approval snapshot', async ({ page }) => {
	const { path, url } = await seed();
	await page.goto(`${url}/1`);
	await expect(page.locator('article.doc')).toContainText('Original bytes');
	await page.getByRole('button', { name: 'Submit for review' }).click();
	await expect(page.getByRole('button', { name: 'Approve', exact: true })).toBeEnabled();
	await request(`${path}/revisions/1/comments`, { content: 'Feedback after rendered snapshot' });
	await page.getByRole('button', { name: 'Approve', exact: true }).click();
	await expect(page.getByRole('alert')).toBeVisible();
	await expect(page.getByRole('button', { name: 'Approve', exact: true })).toBeDisabled();
	await expect(page.locator('[data-comment-id]')).toContainText('Feedback after rendered snapshot');
	expect((await request(`${path}/revisions/1`)).review_status).toBe('in_review');
});

test('issue detail separates explicit pin and layered milestone plus epic context', async ({
	page
}) => {
	const { project, plan, path } = await seed();
	const pid = project.id;
	async function approve(p: string) {
		for (const status of ['in_review', 'approved']) {
			const metadata = await request(p);
			const revision = await request(`${p}/revisions/1`);
			await request(`${p}/revisions/1/decisions`, {
				status,
				expected_head: metadata.head_revision,
				expected_review_version: revision.review_version,
				expected_feedback_version: metadata.feedback_version
			});
		}
	}
	await approve(path);
	const tactical = await request(`/projects/${pid}/plans`, {
		title: 'Tactical design',
		content: '# Tactical bytes'
	});
	await approve(`/projects/${pid}/plans/${tactical.plan.id}`);
	const milestone = await request(`/projects/${pid}/issues`, {
		title: 'Architecture milestone',
		issue_type: 'milestone',
		priority: 2
	});
	const epic = await request(`/projects/${pid}/issues`, {
		title: 'Tactical epic',
		issue_type: 'epic',
		priority: 2
	});
	const task = await request(`/projects/${pid}/issues`, {
		title: 'Implement task',
		issue_type: 'task',
		priority: 2
	});
	await request(`/projects/${pid}/issues/${epic.id}/deps`, {
		depends_on_id: milestone.id,
		type: 'parent-child'
	});
	await request(`/projects/${pid}/issues/${task.id}/deps`, {
		depends_on_id: epic.id,
		type: 'parent-child'
	});
	async function adopt(container: string, planID: string) {
		const issue = await request(`/projects/${pid}/issues/${container}`);
		const project = await request(`/projects/${pid}`);
		const draft = {
			expected_container_version: issue.contract_version,
			expected_governance_generation: project.governance_generation,
			expected_pin: null,
			target_pin: { plan_id: planID, revision: 1 },
			tasks: [],
			container_pins: [],
			follow_ups: [],
			edges: [],
			type_changes: [],
			dry_run: true
		};
		const preview = await request(`/projects/${pid}/issues/${container}/plan-adoption`, draft);
		const tasks = preview.tasks.map(
			(change: { before: { issue_id: string; expected: unknown } }) => ({
				issue_id: change.before.issue_id,
				expected: change.before.expected,
				disposition: 'unchanged',
				reason: 'Existing task agrees with this design',
				follow_up_keys: []
			})
		);
		await request(`/projects/${pid}/issues/${container}/plan-adoption`, {
			...draft,
			tasks,
			dry_run: false
		});
	}
	await adopt(milestone.id, plan.id);
	await adopt(epic.id, tactical.plan.id);
	await request(`${path}/revisions`, {
		content: '# New unadopted architecture',
		expected_revision: 1
	});
	await page.goto(`/${pid}/issues/${task.id}`);
	const context = page.getByRole('region', { name: 'Design context' });
	await expect(context).toContainText('Explicit container pin: None');
	await expect(context).toContainText('Higher-level context');
	await expect(context).toContainText('Primary design');
	await expect(context.getByRole('link', { name: /Server title/ })).toHaveAttribute(
		'href',
		`/${pid}/plans/${plan.id}/1`
	);
	await expect(context.getByRole('link', { name: /Tactical design/ })).toHaveAttribute(
		'href',
		`/${pid}/plans/${tactical.plan.id}/1`
	);
	if (process.env.ARC_UI_EVIDENCE_DIR)
		await context.screenshot({ path: `${process.env.ARC_UI_EVIDENCE_DIR}/layered-issue.png` });
	await context.getByRole('link', { name: /Server title/ }).click();
	await expect(page.locator('article.doc')).toContainText('Original bytes');
	await expect(page.getByTestId('revision-metadata')).toContainText('approved');
	await page.goto(`/${pid}/issues/${epic.id}`);
	await expect(context).toContainText('Explicit container pin:');
	await expect(context.getByRole('link', { name: 'Pinned revision 1' })).toHaveAttribute(
		'href',
		`/${pid}/plans/${tactical.plan.id}/1`
	);
});

test('unlinked issues and typed ambiguous or cyclic governance remain actionable', async ({
	page
}) => {
	const project = await createTestWorkspace();
	const issue = await request(`/projects/${project.id}/issues`, {
		title: 'Legacy issue',
		issue_type: 'task',
		priority: 2
	});
	await page.goto(`/${project.id}/issues/${issue.id}`);
	await expect(page.getByRole('region', { name: 'Design context' })).toContainText(
		'No governing plan'
	);
	for (const code of ['ambiguous_governance', 'governance_cycle']) {
		await page.route(`**/issues/${issue.id}/governing-plan`, (route) =>
			route.fulfill({
				status: 409,
				contentType: 'application/json',
				body: JSON.stringify({ code, error: code })
			})
		);
		await page.reload();
		await expect(page.getByRole('region', { name: 'Design context' })).toContainText(code);
		await expect(page.getByRole('region', { name: 'Design context' })).toContainText('Reconcile');
		await expect(page.getByText('Legacy issue', { exact: true }).first()).toBeVisible();
	}
});

test('legacy ownership resolves beyond the first bounded inventory page without reading paths', async ({
	page
}) => {
	const { project, plan, url } = await seed();
	const calls: string[] = [];
	await page.route('**/api/v1/plans/legacy?*', (route) => {
		calls.push(route.request().url());
		const offset = new URL(route.request().url()).searchParams.get('offset');
		const rows =
			offset === '0'
				? Array.from({ length: 50 }, (_, i) => ({
						id: `plan.other${i}`,
						file_path: '/unreadable/client-only',
						comments: []
					}))
				: [
						{
							id: plan.id,
							imported_project_id: project.id,
							file_path: '/unreadable/client-only',
							comments: []
						}
					];
		return route.fulfill({ contentType: 'application/json', body: JSON.stringify(rows) });
	});
	await page.goto(`/planner/${plan.id}`);
	await expect(page.getByRole('link', { name: 'Open imported plan' })).toHaveAttribute(
		'href',
		`${url}/1`
	);
	expect(calls).toHaveLength(2);
	expect(calls[1]).toContain('limit=50&offset=50');
	await page.getByRole('link', { name: 'Open imported plan' }).click();
	await expect(page.locator('article.doc')).toContainText('Original bytes');
});

test('feedback arriving between snapshot reads requires a deliberate refresh', async ({ page }) => {
	const { path, url } = await seed();
	const metadata = await request(path);
	const revision = await request(`${path}/revisions/1`);
	await request(`${path}/revisions/1/decisions`, {
		status: 'in_review',
		expected_head: 1,
		expected_review_version: revision.review_version,
		expected_feedback_version: metadata.feedback_version
	});
	let inserted = false;
	await page.route(`**/api/v1${path}/revisions/1/comments?*`, async (route) => {
		const snapshot = await route.fetch();
		if (!inserted) {
			inserted = true;
			await request(`${path}/revisions/1/comments`, { content: 'Arrived during load' });
		}
		await route.fulfill({ response: snapshot });
	});
	await page.goto(`${url}/1`);
	await expect(page.locator('article.doc')).toContainText('Original bytes');
	await expect(page.getByRole('alert')).toContainText('changed while loading');
	await expect(page.getByRole('button', { name: 'Approve', exact: true })).toBeDisabled();
	await page.getByRole('button', { name: 'Refresh review' }).click();
	await expect(page.locator('[data-comment-id]')).toContainText('Arrived during load');
	await expect(page.getByRole('button', { name: 'Approve', exact: true })).toBeEnabled();
});

test('uploads exact local bytes explicitly and preserves a rejected disposition reason', async ({
	page
}) => {
	const project = await createTestWorkspace();
	const content = '---\nstatus: approved\n---\n# Browser upload\n\nExact local bytes.\n';
	await page.goto(`/${project.id}/plans`);
	await page.getByText('Upload a new plan', { exact: true }).click();
	await page.getByLabel('Title', { exact: true }).fill('Uploaded from browser');
	await page
		.getByLabel('Markdown file')
		.setInputFiles({ name: 'draft.md', mimeType: 'text/markdown', buffer: Buffer.from(content) });
	await expect(page.getByLabel('Plan content')).toHaveValue(content);
	expect(await request(`/projects/${project.id}/plans`)).toHaveLength(0);
	await page.getByRole('button', { name: 'Upload plan' }).click();
	await expect(page.locator('article.doc')).toContainText('Browser upload');
	await expect(page.getByTestId('revision-metadata')).toContainText('draft');
	const planID = new URL(page.url()).pathname.split('/')[3];
	const path = `/projects/${project.id}/plans/${planID}`;
	expect((await request(`${path}/revisions/1`)).content).toBe(content);
	await page.getByLabel('Overall feedback').fill('Consider deployment');
	await page.getByRole('button', { name: 'Add', exact: true }).click();
	await expect(page.locator('[data-comment-id]')).toContainText('Consider deployment');
	await page.getByRole('button', { name: 'Assess feedback' }).click();
	await page.getByLabel('Disposition reason').fill('Deployment documented in rollout notes');
	await request(`${path}/revisions/1/comments`, { content: 'Concurrent review feedback' });
	await page.getByRole('button', { name: 'Record disposition' }).click();
	await expect(page.getByRole('alert')).toBeVisible();
	await expect(page.getByLabel('Disposition reason')).toHaveValue(
		'Deployment documented in rollout notes'
	);
	await expect(page.getByRole('region', { name: 'Feedback obligations' })).toContainText(
		'Concurrent review feedback'
	);
});

test('outstanding feedback beyond the first page stays visible', async ({ page }) => {
	const { path, url } = await seed();
	const comments = [];
	for (let i = 0; i < 51; i++)
		comments.push(
			await request(`${path}/revisions/1/comments`, { content: `Review obligation ${i}` })
		);
	const queried: string[] = [];
	page.on('request', (req) => {
		if (req.url().includes(`${path}/revisions/1/comments?`)) queried.push(req.url());
	});
	await page.goto(`${url}/1`);
	await expect(page.locator('article.doc')).toContainText('Original bytes');
	await expect(page.locator('[data-feedback-id]')).toHaveCount(51);
	for (const comment of [comments[0], comments[50]])
		await expect(page.locator(`[data-feedback-id="${comment.id}"]`)).toContainText(comment.content);
	expect(queried.some((url) => url.includes('offset=50'))).toBeTruthy();
});
