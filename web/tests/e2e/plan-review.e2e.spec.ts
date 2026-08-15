import { test, expect, type Page } from '@playwright/test';
import { uniqueName } from './fixtures';

const API_BASE = 'http://localhost:7433/api/v1';

/** Rendered document root (PlanRenderer). */
const DOC = 'article.doc';
const MARK = `${DOC} mark[data-anno-id]`;
const CARD = '[data-comment-id]';

/** Appears exactly once in the seed document. */
const UNIQUE = 'canary cohort';
/** Appears twice — once in each of the first two list items. */
const REPEATED = 'run task build';

// Line numbers matter to the anchor assertions below:
//   1 heading · 3 paragraph (UNIQUE) · 5 heading · 7-9 list (REPEATED twice) · 11-13 code fence
const PLAN_MD = `# Rollout Plan

The rollout hinges on a canary cohort before any wider release.

## Build steps

- First, run task build in a clean checkout
- Then, run task build again after codegen
- Finally, publish the release artifacts

\`\`\`bash
docker compose up -d
\`\`\`
`;

/** Same document with the UNIQUE paragraph pushed from line 3 to line 7. */
const MOVED_MD = `# Rollout Plan

## Preamble

We reviewed the incident retro before drafting this.

The rollout hinges on a canary cohort before any wider release.

## Build steps

- First, run task build in a clean checkout
- Then, run task build again after codegen
- Finally, publish the release artifacts
`;

/** Same document with the UNIQUE phrase gone entirely. */
const ORPHAN_MD = `# Rollout Plan

The rollout now ships to every region at once.

## Build steps

- First, run task build in a clean checkout
- Then, run task build again after codegen
- Finally, publish the release artifacts
`;

type SeedAnchor = {
	line_start: number;
	line_end: number;
	quoted_text: string;
	occurrence: number;
};

type SeedComment = {
	id: string;
	content: string;
	line_number: number | null;
	anchor: SeedAnchor | null;
	resolved_at: string | null;
};

/** Anchor covering the UNIQUE phrase in the seed document's paragraph. */
function uniqueAnchor(): SeedAnchor {
	return { line_start: 3, line_end: 3, quoted_text: UNIQUE, occurrence: 0 };
}

/** Anchor covering the FIRST REPEATED phrase; the list renders as one block, lines 7-9. */
function repeatedAnchor(): SeedAnchor {
	return { line_start: 7, line_end: 9, quoted_text: REPEATED, occurrence: 0 };
}

async function postJson<T>(path: string, body: unknown, method = 'POST'): Promise<T> {
	const res = await fetch(`${API_BASE}${path}`, {
		method,
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(body)
	});
	if (!res.ok) {
		throw new Error(`${method} ${path} failed: ${res.status} ${await res.text()}`);
	}
	return res.json();
}

/** Write plan content. The server creates the file (and its parent dirs) on PUT. */
async function writePlanContent(planId: string, content: string): Promise<void> {
	await postJson(`/plans/${planId}`, { content }, 'PUT');
}

/**
 * Register a plan under the server's writable volume and write `content` to it.
 * The file need not exist beforehand — PUT creates it.
 */
async function seedPlan(content: string): Promise<string> {
	const plan = await postJson<{ id: string }>('/plans', {
		file_path: `/data/e2e-plans/${uniqueName('plan')}.md`
	});
	await writePlanContent(plan.id, content);
	return plan.id;
}

async function seedComment(
	planId: string,
	body: { content: string; line_number?: number; anchor?: SeedAnchor }
): Promise<SeedComment> {
	return postJson<SeedComment>(`/plans/${planId}/comments`, body);
}

async function listComments(planId: string): Promise<SeedComment[]> {
	const res = await fetch(`${API_BASE}/plans/${planId}/comments`);
	if (!res.ok) throw new Error(`listComments failed: ${res.status} ${await res.text()}`);
	return res.json();
}

async function getPlanStatus(planId: string): Promise<string> {
	const res = await fetch(`${API_BASE}/plans/${planId}`);
	if (!res.ok) throw new Error(`getPlan failed: ${res.status} ${await res.text()}`);
	return (await res.json()).status;
}

async function openPlan(page: Page, planId: string): Promise<void> {
	await page.goto(`/planner/${planId}`);
	// Markdown rendering is async (shiki); the heading is the readiness signal.
	await expect(page.locator(DOC).getByRole('heading', { name: 'Rollout Plan' })).toBeVisible();
}

/**
 * Select the `occurrence`-th text node containing `text` inside the rendered
 * document. A double-click can't span words and Playwright has no selection
 * API, so the Range is built directly and `selectionchange` — the event the
 * renderer listens on — is dispatched explicitly.
 */
async function selectText(page: Page, text: string, occurrence = 0): Promise<void> {
	await page.evaluate(
		({ containerSel, text, occurrence }) => {
			const root = document.querySelector(containerSel);
			if (!root) throw new Error(`container not found: ${containerSel}`);
			const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
			let seen = 0;
			while (walker.nextNode()) {
				const node = walker.currentNode as Text;
				const idx = node.data.indexOf(text);
				if (idx === -1) continue;
				if (seen++ < occurrence) continue;
				const range = document.createRange();
				range.setStart(node, idx);
				range.setEnd(node, idx + text.length);
				const sel = window.getSelection();
				if (!sel) throw new Error('no selection object');
				sel.removeAllRanges();
				sel.addRange(range);
				document.dispatchEvent(new Event('selectionchange'));
				return;
			}
			throw new Error(`text not found: ${text} (occurrence ${occurrence})`);
		},
		{ containerSel: DOC, text, occurrence }
	);
}

/** Select text, open the composer from the selection pill, and save a comment. */
async function commentOnSelection(
	page: Page,
	text: string,
	body: string,
	occurrence = 0
): Promise<void> {
	await selectText(page, text, occurrence);
	const toolbar = page.locator('.floating-toolbar[role="toolbar"]');
	await expect(toolbar).toBeVisible();
	await toolbar.getByRole('button', { name: 'Comment on selection' }).click();

	const composer = page.locator('.floating-toolbar[role="dialog"]');
	await expect(composer).toBeVisible();
	await composer.locator('textarea').fill(body);
	await composer.getByRole('button', { name: 'Save' }).click();
	await expect(composer).toBeHidden();
}

test.describe('Plan review — unified review flow', () => {
	test('select-to-comment persists a highlight and rail card', async ({ page }) => {
		const planId = await seedPlan(PLAN_MD);
		await openPlan(page, planId);

		await commentOnSelection(page, UNIQUE, 'Confirm the cohort size with SRE.');

		const mark = page.locator(MARK);
		await expect(mark).toHaveCount(1);
		await expect(mark).toHaveText(UNIQUE);

		const card = page.locator(CARD);
		await expect(card).toHaveCount(1);
		await expect(card.locator('.quote')).toContainText(UNIQUE);
		await expect(card).toContainText('Confirm the cohort size with SRE.');

		await page.reload();
		await expect(page.locator(MARK)).toHaveText(UNIQUE);
		await expect(page.locator(CARD)).toContainText('Confirm the cohort size with SRE.');
	});

	test('commenting on the second occurrence anchors to the second list item', async ({ page }) => {
		const planId = await seedPlan(PLAN_MD);
		await openPlan(page, planId);

		await commentOnSelection(page, REPEATED, 'Only the post-codegen build.', 1);

		const stored = await listComments(planId);
		expect(stored).toHaveLength(1);
		expect(stored[0].anchor?.occurrence).toBe(1);

		await page.reload();
		const mark = page.locator(MARK);
		await expect(mark).toHaveCount(1);
		await expect(mark).toHaveText(REPEATED);

		// Both list items share one <ul> block, so line distance cannot separate
		// them — landing on the second item proves the stored occurrence survived
		// the round trip rather than being repaired to the first match.
		const markedItem = await page.evaluate(() => {
			const items = Array.from(document.querySelectorAll('article.doc li'));
			return items.findIndex((li) => li.querySelector('mark[data-anno-id]'));
		});
		expect(markedItem).toBe(1);

		// Resolving to the occurrence that was captured is not drift.
		await expect(mark).not.toHaveClass(/is-drifted/);
		await expect(page.locator(CARD).locator('.badge-drifted')).toHaveCount(0);
	});

	test('editing a comment updates the card and flags it as edited', async ({ page }) => {
		const planId = await seedPlan(PLAN_MD);
		await seedComment(planId, { content: 'Original note', anchor: uniqueAnchor() });
		await openPlan(page, planId);

		const card = page.locator(CARD);
		await expect(card).toContainText('Original note');
		await card.getByRole('button', { name: 'Edit comment' }).click();
		await card.locator('textarea').fill('Revised note');
		await card.getByRole('button', { name: 'Save' }).click();

		await expect(card).toContainText('Revised note');
		await expect(card).toContainText('edited');

		await page.reload();
		await expect(page.locator(CARD)).toContainText('Revised note');
		await expect(page.locator(CARD)).toContainText('edited');
	});

	test('change highlight re-anchors the comment to the next selection', async ({ page }) => {
		const planId = await seedPlan(PLAN_MD);
		const seeded = await seedComment(planId, {
			content: 'Point this at the cohort instead',
			anchor: repeatedAnchor()
		});
		await openPlan(page, planId);
		await expect(page.locator(MARK)).toHaveText(REPEATED);

		await page.locator(CARD).getByRole('button', { name: 'Change highlighted text' }).click();
		await expect(page.locator('.reanchor-banner')).toBeVisible();

		await selectText(page, UNIQUE);
		await expect(page.locator('.reanchor-banner')).toBeHidden();
		await expect(page.locator(MARK)).toHaveText(UNIQUE);
		await expect(page.locator(CARD).locator('.quote')).toContainText(UNIQUE);

		const stored = (await listComments(planId)).find((c) => c.id === seeded.id);
		expect(stored?.anchor?.quoted_text).toBe(UNIQUE);

		await page.reload();
		await expect(page.locator(MARK)).toHaveText(UNIQUE);
	});

	test('resolving hides the card behind the toggle and unresolving restores it', async ({
		page
	}) => {
		const planId = await seedPlan(PLAN_MD);
		await seedComment(planId, { content: 'Looks fine to me', anchor: uniqueAnchor() });
		await openPlan(page, planId);

		await page.locator(CARD).getByRole('button', { name: 'Resolve comment' }).click();
		await expect(page.locator(CARD)).toHaveCount(0);

		const toggle = page.getByRole('button', { name: 'Show resolved (1)' });
		await expect(toggle).toBeVisible();
		await toggle.click();

		await expect(page.getByRole('button', { name: 'Hide resolved (1)' })).toBeVisible();
		await expect(page.locator(CARD)).toHaveClass(/is-resolved/);
		await expect(page.locator(MARK)).toHaveClass(/is-resolved/);

		await page.locator(CARD).getByRole('button', { name: 'Unresolve comment' }).click();
		await expect(page.locator(CARD)).not.toHaveClass(/is-resolved/);
		await expect(page.locator(MARK)).not.toHaveClass(/is-resolved/);
		await expect(page.getByRole('button', { name: /resolved \(/ })).toHaveCount(0);
	});

	test('deleting a comment removes the card, the highlight, and the stored record', async ({
		page
	}) => {
		const planId = await seedPlan(PLAN_MD);
		await seedComment(planId, { content: 'Drop this one', anchor: uniqueAnchor() });
		await openPlan(page, planId);

		const card = page.locator(CARD);
		await card.getByRole('button', { name: 'Delete comment' }).click();
		await card.getByRole('button', { name: 'Yes' }).click();

		await expect(page.locator(CARD)).toHaveCount(0);
		await expect(page.locator(MARK)).toHaveCount(0);
		expect(await listComments(planId)).toHaveLength(0);
	});

	test('overall feedback creates a pinned card with no highlight', async ({ page }) => {
		const planId = await seedPlan(PLAN_MD);
		await openPlan(page, planId);

		await page.locator('#overall-feedback').fill('Ship it after the canary bake.');
		await page.getByRole('button', { name: 'Add' }).click();

		const card = page.locator(CARD);
		await expect(card).toHaveCount(1);
		await expect(card).toContainText('Overall feedback');
		await expect(card).toContainText('Ship it after the canary bake.');
		await expect(page.locator(MARK)).toHaveCount(0);
	});

	test('legacy line-number comment renders without a highlight or console errors', async ({
		page
	}) => {
		const consoleErrors: string[] = [];
		page.on('console', (msg) => {
			if (msg.type() === 'error') consoleErrors.push(msg.text());
		});
		page.on('pageerror', (err) => consoleErrors.push(err.message));

		const planId = await seedPlan(PLAN_MD);
		await seedComment(planId, { content: 'Legacy note', line_number: 3 });
		await openPlan(page, planId);

		const card = page.locator(CARD);
		await expect(card).toContainText('Line 3');
		await expect(card).toContainText('Legacy note');
		await expect(page.locator(MARK)).toHaveCount(0);
		expect(consoleErrors).toEqual([]);
	});

	test('request changes requires a comment and both actions update plan status', async ({
		page
	}) => {
		const planId = await seedPlan(PLAN_MD);
		await openPlan(page, planId);

		const request = page.locator('.btn-request');
		const approve = page.locator('.btn-approve');
		await expect(request).toBeDisabled();

		await page.locator('#overall-feedback').fill('Needs a rollback section.');
		await page.getByRole('button', { name: 'Add' }).click();
		await expect(page.locator(CARD)).toHaveCount(1);
		await expect(request).toBeEnabled();

		await request.click();
		await expect(page.getByText('changes_requested', { exact: true })).toBeVisible();
		expect(await getPlanStatus(planId)).toBe('changes_requested');

		await approve.click();
		await expect(page.getByText('approved', { exact: true })).toBeVisible();
		await expect(approve).toBeDisabled();
		expect(await getPlanStatus(planId)).toBe('approved');
	});

	test('moved text keeps the highlight; removed text orphans the comment', async ({ page }) => {
		const planId = await seedPlan(PLAN_MD);
		await seedComment(planId, { content: 'Check the cohort size', anchor: uniqueAnchor() });
		await openPlan(page, planId);
		await expect(page.locator(MARK)).toHaveText(UNIQUE);

		// The phrase moves from line 3 to line 7 — still found, but drifted.
		await writePlanContent(planId, MOVED_MD);
		await page.reload();
		const mark = page.locator(MARK);
		await expect(mark).toHaveCount(1);
		await expect(mark).toHaveText(UNIQUE);
		await expect(mark).toHaveClass(/is-drifted/);
		await expect(page.locator(CARD).locator('.badge-drifted')).toHaveText('moved');

		// The phrase disappears — no highlight to render, card reports the orphan.
		await writePlanContent(planId, ORPHAN_MD);
		await page.reload();
		await expect(page.locator(MARK)).toHaveCount(0);
		await expect(page.locator(CARD)).toContainText('original text no longer in document');
	});
});
