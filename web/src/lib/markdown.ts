import { Marked } from 'marked';
import { createHighlighter, type Highlighter } from 'shiki';
import markedShiki from 'marked-shiki';
import DOMPurify from 'isomorphic-dompurify';

let highlighterPromise: Promise<Highlighter> | null = null;
let configuredMarked: Marked | null = null;

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
	'text'
];
const THEME = 'github-dark-dimmed';

async function getMarkedInstance(): Promise<Marked> {
	if (configuredMarked) {
		return configuredMarked;
	}

	if (!highlighterPromise) {
		highlighterPromise = createHighlighter({
			themes: [THEME],
			langs: LANGUAGES
		});
	}

	const highlighter = await highlighterPromise;

	const marked = new Marked();
	marked.setOptions({
		gfm: true,
		breaks: true
	});

	const loadedLangs = new Set(highlighter.getLoadedLanguages());

	marked.use(
		markedShiki({
			highlight(code, lang) {
				const resolved = lang && loadedLangs.has(lang) ? lang : 'text';
				return highlighter.codeToHtml(code, {
					lang: resolved,
					theme: THEME
				});
			}
		})
	);

	configuredMarked = marked;
	return configuredMarked;
}

/**
 * Render a raw markdown string to sanitized HTML ready for {@html}.
 * Lazily initializes a shiki highlighter on first call and caches it.
 */
export async function renderMarkdown(content: string): Promise<string> {
	if (!content) {
		return '';
	}

	const marked = await getMarkedInstance();
	const html = await marked.parse(content);

	return DOMPurify.sanitize(html, {
		ADD_TAGS: ['span'],
		ADD_ATTR: ['style', 'class']
	});
}

/**
 * Strip markdown syntax from a string to produce plain text.
 * Useful for card preview snippets.
 */
export function stripMarkdown(content: string): string {
	if (!content) {
		return '';
	}

	return (
		content
			// Remove code fences and their content
			.replace(/```[\s\S]*?```/g, '')
			// Remove image syntax ![alt](url) -> alt
			.replace(/!\[([^\]]*)\]\([^)]*\)/g, '$1')
			// Remove link syntax [text](url) -> text
			.replace(/\[([^\]]*)\]\([^)]*\)/g, '$1')
			// Remove heading markers
			.replace(/^#{1,6}\s+/gm, '')
			// Remove bold/italic with double markers first (**bold** or __bold__)
			.replace(/(\*\*|__)(.*?)\1/g, '$2')
			// Remove italic with single markers (*italic* or _italic_)
			.replace(/(\*|_)(.*?)\1/g, '$2')
			// Collapse multiple newlines to single space
			.replace(/\n{2,}/g, ' ')
			// Replace remaining newlines with space
			.replace(/\n/g, ' ')
			// Trim whitespace
			.trim()
	);
}
