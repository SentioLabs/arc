import type { components } from '$lib/api/types';

type Status = components['schemas']['Status'];
type IssueType = components['schemas']['IssueType'];

export interface IssueFilters {
	status?: Status;
	issueType?: IssueType;
	priority?: number;
	assignee?: string;
	q?: string;
}

// Using Svelte 5 runes for reactive state
function createFilterStore() {
	let filters = $state<IssueFilters>({});

	return {
		get filters() {
			return filters;
		},
		setStatus(status: Status | undefined) {
			filters = { ...filters, status };
		},
		setIssueType(issueType: IssueType | undefined) {
			filters = { ...filters, issueType };
		},
		setPriority(priority: number | undefined) {
			filters = { ...filters, priority };
		},
		setAssignee(assignee: string | undefined) {
			filters = { ...filters, assignee };
		},
		setQuery(q: string | undefined) {
			filters = { ...filters, q };
		},
		clear() {
			filters = {};
		},
		// Convert to URL search params for navigation
		toSearchParams(): URLSearchParams {
			const params = new URLSearchParams();
			if (filters.status) params.set('status', filters.status);
			if (filters.issueType) params.set('type', filters.issueType);
			if (filters.priority !== undefined) params.set('priority', filters.priority.toString());
			if (filters.assignee) params.set('assignee', filters.assignee);
			if (filters.q) params.set('q', filters.q);
			return params;
		},
		// Initialize from URL search params
		fromSearchParams(params: URLSearchParams) {
			filters = {
				status: (params.get('status') as Status) || undefined,
				issueType: (params.get('type') as IssueType) || undefined,
				priority: params.has('priority') ? parseInt(params.get('priority')!) : undefined,
				assignee: params.get('assignee') || undefined,
				q: params.get('q') || undefined
			};
		}
	};
}

export const filterStore = createFilterStore();
