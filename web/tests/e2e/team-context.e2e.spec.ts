import {
	test,
	expect,
	createTestIssue,
	createTestLabel,
	deleteTestLabel,
	addLabelToIssue,
	uniqueName,
	addTestDependency,
} from './fixtures';

test.describe('Team Context Page', () => {
	test('team page loads with heading visible', async ({ page, testWorkspace }) => {
		await page.goto(`/${testWorkspace.id}/teams`);
		await expect(page.getByRole('heading', { name: 'Teams' })).toBeVisible();
	});

	test('shows empty state when no teammate labels exist', async ({ page, testWorkspace }) => {
		await page.goto(`/${testWorkspace.id}/teams`);
		await expect(page.getByText('No team assignments')).toBeVisible();
		await expect(page.getByText('teammate:role')).toBeVisible();
	});

	test('displays issues in role lanes with teammate labels', async ({ page, testWorkspace }) => {
		const wsId = testWorkspace.id;
		const feLabelName = uniqueName('teammate:frontend');
		const beLabelName = uniqueName('teammate:backend');
		const labelsToClean: string[] = [];

		try {
			// Create teammate labels
			await createTestLabel(feLabelName);
			labelsToClean.push(feLabelName);
			await createTestLabel(beLabelName);
			labelsToClean.push(beLabelName);

			// Create issues with teammate labels
			const feIssue = await createTestIssue(wsId, {
				title: 'FE task for alice',
				issue_type: 'task',
			});
			const beIssue = await createTestIssue(wsId, {
				title: 'BE task for bob',
				issue_type: 'task',
			});

			// Add teammate labels to issues
			await addLabelToIssue(wsId, feIssue.id as string, feLabelName);
			await addLabelToIssue(wsId, beIssue.id as string, beLabelName);

			// Navigate to teams page
			await page.goto(`/${wsId}/teams`);

			// Verify both issues are visible
			await expect(page.getByText('FE task for alice')).toBeVisible();
			await expect(page.getByText('BE task for bob')).toBeVisible();

			// Verify role lane headers are present (role names extracted from label after "teammate:")
			// The label names are like "teammate:frontend-<timestamp>-<random>" so the role
			// is the full part after "teammate:"
			const feRole = feLabelName.replace('teammate:', '');
			const beRole = beLabelName.replace('teammate:', '');
			await expect(page.getByText(feRole, { exact: false })).toBeVisible();
			await expect(page.getByText(beRole, { exact: false })).toBeVisible();
		} finally {
			for (const label of labelsToClean) {
				await deleteTestLabel(label).catch(() => {});
			}
		}
	});

	test('filters issues by epic selection', async ({ page, testWorkspace }) => {
		const wsId = testWorkspace.id;
		const labelName = uniqueName('teammate:dev');
		const labelsToClean: string[] = [];

		try {
			// Create a teammate label
			await createTestLabel(labelName);
			labelsToClean.push(labelName);

			// Create an epic
			const epic = await createTestIssue(wsId, {
				title: 'Test Epic for filtering',
				issue_type: 'epic',
			});

			// Create a child task with teammate label
			const childTask = await createTestIssue(wsId, {
				title: 'Child task under epic',
				issue_type: 'task',
			});

			// Set parent-child dependency (child depends on parent)
			await addTestDependency(wsId, childTask.id as string, epic.id as string, 'parent-child');

			// Add teammate label to child
			await addLabelToIssue(wsId, childTask.id as string, labelName);

			// Navigate to teams page
			await page.goto(`/${wsId}/teams`);

			// The epic selector should be available with "All teammate issues" default
			await expect(page.getByText('Scope:')).toBeVisible();

			// Open the custom Select dropdown by clicking the trigger button
			const selectTrigger = page.locator('button[aria-haspopup="listbox"]').filter({ hasText: 'All teammate issues' });
			await selectTrigger.click();

			// Click the epic option in the dropdown
			const epicOptionText = `${epic.id as string}: Test Epic for filtering`;
			await page.getByRole('option', { name: epicOptionText }).click();

			// Wait for filtered view - the child task should still be visible
			await expect(page.getByText('Child task under epic')).toBeVisible();
		} finally {
			for (const label of labelsToClean) {
				await deleteTestLabel(label).catch(() => {});
			}
		}
	});

	test('navigates to issue detail when clicking a card', async ({ page, testWorkspace }) => {
		const wsId = testWorkspace.id;
		const labelName = uniqueName('teammate:nav');
		const labelsToClean: string[] = [];

		try {
			// Create teammate label
			await createTestLabel(labelName);
			labelsToClean.push(labelName);

			// Create issue with teammate label
			const issue = await createTestIssue(wsId, {
				title: 'Clickable team issue',
				issue_type: 'task',
			});
			const issueId = issue.id as string;
			await addLabelToIssue(wsId, issueId, labelName);

			// Navigate to teams page
			await page.goto(`/${wsId}/teams`);

			// Wait for the issue to appear
			await expect(page.getByText('Clickable team issue')).toBeVisible();

			// Click the issue card (TeamIssueCard is an <a> tag)
			await page.getByText('Clickable team issue').click();

			// Should navigate to the issue detail page
			await page.waitForURL(`**/${wsId}/issues/${issueId}`);
			await expect(page).toHaveURL(new RegExp(`/${wsId}/issues/${issueId}`));
		} finally {
			for (const label of labelsToClean) {
				await deleteTestLabel(label).catch(() => {});
			}
		}
	});
});
