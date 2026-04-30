import { test as base, expect } from '@playwright/test';

const API_BASE = 'http://localhost:7433/api/v1';

/** Generate a unique name with a prefix for test isolation. */
export function uniqueName(prefix = 'test'): string {
	return `${prefix}-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
}

/** Create a project via the API. Returns the created project object. */
export async function createTestWorkspace(
	name?: string,
	opts?: { path?: string; prefix?: string; description?: string }
): Promise<{ id: string; name: string; prefix: string }> {
	const wsName = name ?? uniqueName('ws');
	// Generate a short unique prefix from the name (uppercase, max 4 chars)
	const defaultPrefix =
		wsName
			.replace(/[^a-zA-Z]/g, '')
			.slice(0, 4)
			.toUpperCase() || 'TE';
	const res = await fetch(`${API_BASE}/projects`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({
			name: wsName,
			path: opts?.path ?? `/tmp/test-${wsName}`,
			prefix: opts?.prefix ?? defaultPrefix,
			description: opts?.description
		})
	});
	if (!res.ok) {
		throw new Error(`createTestWorkspace failed: ${res.status} ${await res.text()}`);
	}
	return res.json();
}

/** Delete a project by ID. */
export async function deleteTestWorkspace(id: string): Promise<void> {
	const res = await fetch(`${API_BASE}/projects/${id}`, { method: 'DELETE' });
	if (!res.ok) {
		throw new Error(`deleteTestWorkspace failed: ${res.status} ${await res.text()}`);
	}
}

/** Create an issue in a project. Returns the created issue object. */
export async function createTestIssue(
	wsId: string,
	opts?: {
		title?: string;
		issue_type?: string;
		priority?: number;
		description?: string;
	}
): Promise<{
	id: string;
	title: string;
	status: string;
	priority: number;
	[key: string]: unknown;
}> {
	const res = await fetch(`${API_BASE}/projects/${wsId}/issues`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({
			title: opts?.title ?? uniqueName('issue'),
			issue_type: opts?.issue_type,
			priority: opts?.priority,
			description: opts?.description
		})
	});
	if (!res.ok) {
		throw new Error(`createTestIssue failed: ${res.status} ${await res.text()}`);
	}
	return res.json();
}

/** Update an issue. Returns the updated issue object. */
export async function updateTestIssue(
	wsId: string,
	issueId: string,
	fields: Record<string, unknown>
): Promise<{ id: string; [key: string]: unknown }> {
	const res = await fetch(`${API_BASE}/projects/${wsId}/issues/${issueId}`, {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(fields)
	});
	if (!res.ok) {
		throw new Error(`updateTestIssue failed: ${res.status} ${await res.text()}`);
	}
	return res.json();
}

/** Create a label. Returns the created label object. */
export async function createTestLabel(
	name?: string,
	opts?: { color?: string; description?: string }
): Promise<{ name: string; color?: string; description?: string }> {
	const labelName = name ?? uniqueName('label');
	const res = await fetch(`${API_BASE}/labels`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({
			name: labelName,
			color: opts?.color,
			description: opts?.description
		})
	});
	if (!res.ok) {
		throw new Error(`createTestLabel failed: ${res.status} ${await res.text()}`);
	}
	return res.json();
}

/** Delete a label by name. */
export async function deleteTestLabel(name: string): Promise<void> {
	const res = await fetch(`${API_BASE}/labels/${encodeURIComponent(name)}`, {
		method: 'DELETE'
	});
	if (!res.ok) {
		throw new Error(`deleteTestLabel failed: ${res.status} ${await res.text()}`);
	}
}

/** Add a dependency between issues. */
export async function addTestDependency(
	wsId: string,
	issueId: string,
	dependsOnId: string,
	type?: string
): Promise<{ issue_id: string; depends_on_id: string; [key: string]: unknown }> {
	const res = await fetch(`${API_BASE}/projects/${wsId}/issues/${issueId}/deps`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({
			depends_on_id: dependsOnId,
			type: type
		})
	});
	if (!res.ok) {
		throw new Error(`addTestDependency failed: ${res.status} ${await res.text()}`);
	}
	return res.json();
}

/** Add a comment to an issue. */
export async function addTestComment(
	wsId: string,
	issueId: string,
	text: string,
	author?: string
): Promise<{ id: number; text: string; [key: string]: unknown }> {
	const res = await fetch(`${API_BASE}/projects/${wsId}/issues/${issueId}/comments`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ text, author })
	});
	if (!res.ok) {
		throw new Error(`addTestComment failed: ${res.status} ${await res.text()}`);
	}
	return res.json();
}

/** Add a label to an issue. */
export async function addLabelToIssue(
	wsId: string,
	issueId: string,
	labelName: string
): Promise<void> {
	const res = await fetch(`${API_BASE}/projects/${wsId}/issues/${issueId}/labels`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ label: labelName })
	});
	if (!res.ok) {
		throw new Error(`addLabelToIssue failed: ${res.status} ${await res.text()}`);
	}
}

// ── Playwright fixture: auto-creates a project per test ──

type TestFixtures = {
	testWorkspace: { id: string; name: string; prefix: string };
};

export const test = base.extend<TestFixtures>({
	// biome-ignore lint/correctness/noEmptyPattern: playwright fixture API requires destructured parameter
	testWorkspace: async ({}, use) => {
		const ws = await createTestWorkspace();
		await use(ws);
		await deleteTestWorkspace(ws.id);
	}
});

export { expect };
