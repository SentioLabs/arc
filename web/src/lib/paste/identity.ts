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

export function parseShareFragment(hash: string): { k: string | null; t: string | null } {
	const raw = hash.startsWith('#') ? hash.slice(1) : hash;
	const params = new URLSearchParams(raw);
	const k = params.get('k');
	const t = params.get('t');
	return {
		k: k && k.length > 0 ? k : null,
		t: t && t.length > 0 ? t : null
	};
}
