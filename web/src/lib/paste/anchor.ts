import type { Anchor } from './types';

export type AnchorResolution = {
	line_start: number;
	line_end: number;
	char_start?: number;
	char_end?: number;
	status: 'ok' | 'drifted' | 'orphaned';
};

export function resolveAnchor(plan: string, anchor: Anchor): AnchorResolution {
	const lines = plan.split('\n');

	if (anchor.line_start <= lines.length && anchor.line_end <= lines.length) {
		const slice = lines.slice(anchor.line_start - 1, anchor.line_end).join('\n');
		if (slice.includes(anchor.quoted_text)) {
			return {
				line_start: anchor.line_start,
				line_end: anchor.line_end,
				char_start: anchor.char_start,
				char_end: anchor.char_end,
				status: 'ok'
			};
		}
	}

	if (anchor.heading_slug) {
		const headingIdx = findHeadingIndex(lines, anchor.heading_slug);
		if (headingIdx >= 0) {
			const window = lines.slice(headingIdx, Math.min(headingIdx + 50, lines.length)).join('\n');
			const offset = window.indexOf(anchor.quoted_text);
			if (offset >= 0) {
				const lineNum = headingIdx + 1 + countNewlinesBefore(window, offset);
				return {
					line_start: lineNum,
					line_end: lineNum + countNewlinesBefore(anchor.quoted_text, anchor.quoted_text.length),
					status: 'drifted'
				};
			}
		}
	}

	if (anchor.context_before && anchor.context_after) {
		const needle = anchor.context_before + anchor.quoted_text + anchor.context_after;
		const idx = plan.indexOf(needle);
		if (idx >= 0) {
			const lineNum = countNewlinesBefore(plan, idx + (anchor.context_before?.length ?? 0)) + 1;
			return {
				line_start: lineNum,
				line_end: lineNum + countNewlinesBefore(anchor.quoted_text, anchor.quoted_text.length),
				status: 'drifted'
			};
		}
	}

	return { line_start: anchor.line_start, line_end: anchor.line_end, status: 'orphaned' };
}

function findHeadingIndex(lines: string[], slug: string): number {
	for (let i = 0; i < lines.length; i++) {
		const m = lines[i].match(/^#+\s+(.*)$/);
		if (m && slugify(m[1]) === slug) return i;
	}
	return -1;
}

export function slugify(text: string): string {
	return text
		.toLowerCase()
		.replace(/[^a-z0-9\s-]/g, '')
		.trim()
		.replace(/\s+/g, '-');
}

function countNewlinesBefore(s: string, idx: number): number {
	let n = 0;
	for (let i = 0; i < idx && i < s.length; i++) if (s[i] === '\n') n++;
	return n;
}
