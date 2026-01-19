import type { components } from '$lib/api/types';

type Status = components['schemas']['Status'];
type IssueType = components['schemas']['IssueType'];
type DependencyType = components['schemas']['DependencyType'];

// Status utilities
export const statusLabels: Record<Status, string> = {
	open: 'Open',
	in_progress: 'In Progress',
	blocked: 'Blocked',
	deferred: 'Deferred',
	closed: 'Closed'
};

export const statusColors: Record<Status, string> = {
	open: 'bg-status-open text-white',
	in_progress: 'bg-status-in-progress text-white',
	blocked: 'bg-status-blocked text-white',
	deferred: 'bg-status-deferred text-gray-900',
	closed: 'bg-status-closed text-white'
};

// Priority utilities (0 = critical, 4 = backlog)
export const priorityLabels: Record<number, string> = {
	0: 'Critical',
	1: 'High',
	2: 'Medium',
	3: 'Low',
	4: 'Backlog'
};

export const priorityColors: Record<number, string> = {
	0: 'bg-priority-critical text-white',
	1: 'bg-priority-high text-white',
	2: 'bg-priority-medium text-gray-900',
	3: 'bg-priority-low text-gray-900',
	4: 'bg-priority-none text-white'
};

// Issue type utilities
export const issueTypeLabels: Record<IssueType, string> = {
	bug: 'Bug',
	feature: 'Feature',
	task: 'Task',
	epic: 'Epic',
	chore: 'Chore'
};

export const issueTypeIcons: Record<IssueType, string> = {
	bug: '🐛',
	feature: '✨',
	task: '📋',
	epic: '🏔️',
	chore: '🔧'
};

// Dependency type utilities
export const dependencyTypeLabels: Record<DependencyType, string> = {
	blocks: 'Blocks',
	'parent-child': 'Parent of',
	related: 'Related to',
	'discovered-from': 'Discovered from'
};

// Date formatting
export function formatDate(dateString: string): string {
	const date = new Date(dateString);
	return date.toLocaleDateString('en-US', {
		month: 'short',
		day: 'numeric',
		year: date.getFullYear() !== new Date().getFullYear() ? 'numeric' : undefined
	});
}

export function formatDateTime(dateString: string): string {
	const date = new Date(dateString);
	return date.toLocaleDateString('en-US', {
		month: 'short',
		day: 'numeric',
		year: 'numeric',
		hour: 'numeric',
		minute: '2-digit'
	});
}

export function formatRelativeTime(dateString: string): string {
	const date = new Date(dateString);
	const now = new Date();
	const diffMs = now.getTime() - date.getTime();
	const diffMins = Math.floor(diffMs / 60000);
	const diffHours = Math.floor(diffMs / 3600000);
	const diffDays = Math.floor(diffMs / 86400000);

	if (diffMins < 1) return 'just now';
	if (diffMins < 60) return `${diffMins}m ago`;
	if (diffHours < 24) return `${diffHours}h ago`;
	if (diffDays < 7) return `${diffDays}d ago`;
	return formatDate(dateString);
}
