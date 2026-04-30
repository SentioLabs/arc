const KEY = 'arc.reviewer.name';

export function getReviewerName(): string | null {
	if (typeof localStorage === 'undefined') return null;
	return localStorage.getItem(KEY);
}

export function setReviewerName(name: string): void {
	if (typeof localStorage === 'undefined') return;
	localStorage.setItem(KEY, name.trim());
}

export function clearReviewerName(): void {
	if (typeof localStorage === 'undefined') return;
	localStorage.removeItem(KEY);
}
