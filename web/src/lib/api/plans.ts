import type { components } from './types';

type Schema = components['schemas'];
export type Plan = Schema['Plan'];
export type Revision = Schema['PlanRevisionWithContent'];
export type Comment = Schema['PlanComment'];
export type Disposition = Schema['PlanFeedbackDisposition'];
export type Governance = Schema['GoverningPlan'];

export class PlanAPIError extends Error {
	constructor(
		message: string,
		public status: number,
		public code?: string
	) {
		super(message);
	}
}

async function request<T>(path: string, method = 'GET', body?: unknown, key?: string): Promise<T> {
	const response = await fetch(`/api/v1${path}`, {
		method,
		headers: { 'Content-Type': 'application/json', ...(key ? { 'Idempotency-Key': key } : {}) },
		...(body === undefined ? {} : { body: JSON.stringify(body) })
	});
	const data = await response.json();
	if (!response.ok)
		throw new PlanAPIError(
			data.error ?? `Request failed (${response.status})`,
			response.status,
			data.code
		);
	return data;
}
const planPath = (project: string, plan: string) =>
	`/projects/${encodeURIComponent(project)}/plans/${encodeURIComponent(plan)}`;
const revisionPath = (project: string, plan: string, revision: number) =>
	`${planPath(project, plan)}/revisions/${revision}`;
export const revisionURL = (project: string, plan: string, revision: number) =>
	`/${encodeURIComponent(project)}/plans/${encodeURIComponent(plan)}/${revision}`;
export const listPlans = (project: string, archived: boolean, offset = 0) =>
	request<Plan[]>(
		`/projects/${encodeURIComponent(project)}/plans?archived=${archived}&limit=50&offset=${offset}`
	);
export const getMetadata = (project: string, plan: string) =>
	request<Plan>(planPath(project, plan));
export const getRevision = (project: string, plan: string, revision: number) =>
	request<Revision>(revisionPath(project, plan, revision));
export const listRevisions = (project: string, plan: string, offset = 0) =>
	request<Schema['PlanRevision'][]>(
		`${planPath(project, plan)}/revisions?limit=50&offset=${offset}`
	);
export const saveRevision = (
	project: string,
	plan: string,
	body: Schema['PlanSave'],
	key: string
) => request<Schema['PlanWriteResult']>(`${planPath(project, plan)}/revisions`, 'POST', body, key);
export const uploadPlan = (project: string, body: Schema['PlanUpload'], key: string) =>
	request<Schema['PlanWriteResult']>(
		`/projects/${encodeURIComponent(project)}/plans`,
		'POST',
		body,
		key
	);
export const updateMetadata = (project: string, plan: string, body: Schema['PlanMetadataUpdate']) =>
	request<Plan>(planPath(project, plan), 'PATCH', body);
export const decideRevision = (
	project: string,
	plan: string,
	revision: number,
	body: Schema['PlanReviewRequest']
) =>
	request<Schema['PlanRevision']>(
		`${revisionPath(project, plan, revision)}/decisions`,
		'POST',
		body
	);
export const listComments = (project: string, plan: string, revision: number, offset = 0) =>
	request<Comment[]>(
		`${revisionPath(project, plan, revision)}/comments?include_prior=true&limit=50&offset=${offset}`
	);
export const addComment = (
	project: string,
	plan: string,
	revision: number,
	body: Schema['CreatePlanCommentRequest']
) => request<Comment>(`${revisionPath(project, plan, revision)}/comments`, 'POST', body);
export const mutateComment = (
	project: string,
	plan: string,
	revision: number,
	comment: string,
	body: Schema['PlanCommentUpdate'],
	remove = false
) =>
	request<Comment>(
		`${revisionPath(project, plan, revision)}/comments/${encodeURIComponent(comment)}`,
		remove ? 'DELETE' : 'PATCH',
		body
	);
export const listDispositions = (project: string, plan: string, revision: number, offset = 0) =>
	request<Disposition[]>(
		`${revisionPath(project, plan, revision)}/dispositions?limit=50&offset=${offset}`
	);
export const addDisposition = (
	project: string,
	plan: string,
	revision: number,
	body: Schema['PlanDispositionRequest']
) => request<Disposition>(`${revisionPath(project, plan, revision)}/dispositions`, 'POST', body);
export const listDecisions = (project: string, plan: string, revision: number, offset = 0) =>
	request<Schema['PlanReviewEvent'][]>(
		`${revisionPath(project, plan, revision)}/decisions?limit=50&offset=${offset}`
	);
export const listCommentVersions = (
	project: string,
	plan: string,
	revision: number,
	comment: string,
	offset = 0
) =>
	request<Schema['PlanCommentVersion'][]>(
		`${revisionPath(project, plan, revision)}/comments/${encodeURIComponent(comment)}/versions?limit=50&offset=${offset}`
	);
export const listLegacyPlans = (offset = 0) =>
	request<Schema['LegacyPlanInventory'][]>(`/plans/legacy?limit=50&offset=${offset}`);
export const resolveGovernance = (project: string, issue: string) =>
	request<Governance | null>(
		`/projects/${encodeURIComponent(project)}/issues/${encodeURIComponent(issue)}/governing-plan`
	);

// Follow bounded pages so feedback beyond the first page cannot disappear from a review.
export async function allPages<T>(read: (offset: number) => Promise<T[]>): Promise<T[]> {
	const result: T[] = [];
	for (let offset = 0; ; offset += 50) {
		const page = await read(offset);
		result.push(...page);
		if (page.length < 50) return result;
	}
}
