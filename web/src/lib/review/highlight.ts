import { createHighlighter, type Highlighter } from 'shiki';
import type { DiffFile } from 'diff2html/lib/types';
import { detectLanguage } from './parser';

const THEME = 'github-dark-dimmed';
const LANGUAGES = [
	'go',
	'typescript',
	'javascript',
	'json',
	'bash',
	'sql',
	'yaml',
	'markdown',
	'html',
	'css',
	'python',
	'shell',
	'text',
	'toml',
	'rust',
	'svelte'
];

let highlighterPromise: Promise<Highlighter> | null = null;

function getHighlighter(): Promise<Highlighter> {
	if (!highlighterPromise) {
		highlighterPromise = createHighlighter({
			themes: [THEME],
			langs: LANGUAGES
		});
	}
	return highlighterPromise;
}

/**
 * Highlight all lines in a diff file.
 * Returns a Map from raw line content (without +/-/space prefix) to highlighted HTML.
 *
 * Strategy:
 * 1. Collect all unique line contents from the diff (strip the +/-/space prefix)
 * 2. Join them as a single "file" and highlight with Shiki
 * 3. Split the output HTML by lines and map back
 */
export async function highlightDiffFile(file: DiffFile): Promise<Map<string, string>> {
	const highlighter = await getHighlighter();
	const lang = detectLanguage(file.newName || file.oldName);

	// Collect all unique line contents (stripped of diff prefix)
	const lines: string[] = [];
	const seen = new Set<string>();

	for (const block of file.blocks) {
		for (const line of block.lines) {
			// Strip the leading +/- or space character
			const content = line.content.slice(1);
			if (!seen.has(content)) {
				seen.add(content);
				lines.push(content);
			}
		}
	}

	if (lines.length === 0) {
		return new Map();
	}

	// Highlight the combined content as a single file
	const combined = lines.join('\n');
	const html = highlighter.codeToHtml(combined, {
		lang: lang,
		theme: THEME
	});

	// Parse the highlighted HTML to extract individual lines
	// Shiki wraps each line in a <span class="line">...</span>
	const lineRegex = /<span class="line">(.*?)<\/span>/gs;
	const highlightedLines: string[] = [];
	let match;
	while ((match = lineRegex.exec(html)) !== null) {
		highlightedLines.push(match[1]);
	}

	// Map original content back to highlighted HTML
	const result = new Map<string, string>();
	for (let i = 0; i < lines.length && i < highlightedLines.length; i++) {
		result.set(lines[i], highlightedLines[i]);
	}

	return result;
}

/**
 * Get the stripped content of a diff line (without the +/-/space prefix).
 */
export function getLineContent(line: { content: string }): string {
	return line.content.slice(1);
}
