import type { components } from '$lib/api/types';

type Status = components['schemas']['Status'];
type IssueType = components['schemas']['IssueType'];

export interface IssueFilters {
	statuses?: Status[];
	issueTypes?: IssueType[];
	priorities?: number[];
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
		setStatuses(statuses: Status[] | undefined) {
			filters = { ...filters, statuses };
		},
		setIssueTypes(issueTypes: IssueType[] | undefined) {
			filters = { ...filters, issueTypes };
		},
		setPriorities(priorities: number[] | undefined) {
			filters = { ...filters, priorities };
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
			if (filters.statuses) {
				for (const s of filters.statuses) {
					params.append('status', s);
				}
			}
			if (filters.issueTypes) {
				for (const t of filters.issueTypes) {
					params.append('type', t);
				}
			}
			if (filters.priorities) {
				for (const p of filters.priorities) {
					params.append('priority', p.toString());
				}
			}
			if (filters.assignee) params.set('assignee', filters.assignee);
			if (filters.q) params.set('q', filters.q);
			return params;
		},
		// Initialize from URL search params
		fromSearchParams(params: URLSearchParams) {
			const statusValues = params.getAll('status');
			const typeValues = params.getAll('type');
			const priorityValues = params.getAll('priority');
			filters = {
				statuses: statusValues.length > 0 ? (statusValues as Status[]) : undefined,
				issueTypes: typeValues.length > 0 ? (typeValues as IssueType[]) : undefined,
				priorities:
					priorityValues.length > 0
						? priorityValues.map((p) => parseInt(p, 10))
						: undefined,
				assignee: params.get('assignee') || undefined,
				q: params.get('q') || undefined
			};
		}
	};
}

export const filterStore = createFilterStore();
