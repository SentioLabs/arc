import { test, expect, createTestIssue, addTestDependency } from './fixtures';

test.describe('Dependencies on Issue Detail Page', () => {
	test('dependency shown on detail', async ({ page, testWorkspace }) => {
		const issueA = await createTestIssue(testWorkspace.id, { title: 'Issue A' });
		const issueB = await createTestIssue(testWorkspace.id, { title: 'Issue B' });
		await addTestDependency(testWorkspace.id, issueA.id as string, issueB.id as string, 'blocks');

		await page.goto(`/${testWorkspace.id}/issues/${issueA.id}`);

		// Verify issueB's ID is visible in the Dependencies section
		const depsSection = page.locator('section', { hasText: /^Dependencies/ });
		await expect(depsSection.getByText(issueB.id as string)).toBeVisible();
	});

	test('dependent shown on blocker', async ({ page, testWorkspace }) => {
		const issueA = await createTestIssue(testWorkspace.id, { title: 'Issue A' });
		const issueB = await createTestIssue(testWorkspace.id, { title: 'Issue B' });
		await addTestDependency(testWorkspace.id, issueA.id as string, issueB.id as string, 'blocks');

		// Navigate to issueB — it should show issueA as a dependent
		await page.goto(`/${testWorkspace.id}/issues/${issueB.id}`);

		const dependentsSection = page.locator('section', { hasText: /^Dependents/ });
		await expect(dependentsSection.getByText(issueA.id as string)).toBeVisible();
	});

	test('no dependencies empty state', async ({ page, testWorkspace }) => {
		const issue = await createTestIssue(testWorkspace.id, { title: 'Standalone Issue' });

		await page.goto(`/${testWorkspace.id}/issues/${issue.id}`);

		await expect(page.getByText('No dependencies')).toBeVisible();
	});

	test('multiple dependencies', async ({ page, testWorkspace }) => {
		const issueA = await createTestIssue(testWorkspace.id, { title: 'Issue A' });
		const issueB = await createTestIssue(testWorkspace.id, { title: 'Issue B' });
		const issueC = await createTestIssue(testWorkspace.id, { title: 'Issue C' });

		await addTestDependency(testWorkspace.id, issueA.id as string, issueB.id as string, 'blocks');
		await addTestDependency(testWorkspace.id, issueA.id as string, issueC.id as string, 'blocks');

		await page.goto(`/${testWorkspace.id}/issues/${issueA.id}`);

		const depsSection = page.locator('section', { hasText: /^Dependencies/ });
		await expect(depsSection.getByText(issueB.id as string)).toBeVisible();
		await expect(depsSection.getByText(issueC.id as string)).toBeVisible();
	});

	test('dependency links clickable', async ({ page, testWorkspace }) => {
		const issueA = await createTestIssue(testWorkspace.id, { title: 'Issue A' });
		const issueB = await createTestIssue(testWorkspace.id, { title: 'Issue B' });
		await addTestDependency(testWorkspace.id, issueA.id as string, issueB.id as string, 'blocks');

		await page.goto(`/${testWorkspace.id}/issues/${issueA.id}`);

		// Click the dependency link for issueB
		const depsSection = page.locator('section', { hasText: /^Dependencies/ });
		const depLink = depsSection.locator('a', { hasText: issueB.id as string });
		await depLink.click();

		// Verify navigation to issueB's detail page
		await expect(page).toHaveURL(new RegExp(`/${testWorkspace.id}/issues/${issueB.id}`));
		// Verify we're on issueB's page
		await expect(page.getByText('Issue B')).toBeVisible();
	});
});
