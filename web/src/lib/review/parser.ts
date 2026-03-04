import { parse } from 'diff2html';
import type { DiffFile, DiffBlock, DiffLine, LineType } from 'diff2html/lib/types';

export type { DiffFile, DiffBlock, DiffLine, LineType };

export interface ParsedDiff {
	files: DiffFile[];
	stats: {
		filesChanged: number;
		totalAdditions: number;
		totalDeletions: number;
	};
}

export function parseDiff(rawDiff: string): ParsedDiff {
	const files = parse(rawDiff);
	const stats = {
		filesChanged: files.length,
		totalAdditions: files.reduce((sum, f) => sum + f.addedLines, 0),
		totalDeletions: files.reduce((sum, f) => sum + f.deletedLines, 0)
	};
	return { files, stats };
}

/**
 * Get the display filename for a diff file.
 * Handles renames (old -> new) and new/deleted files.
 */
export function getFileName(file: DiffFile): string {
	if (file.isDeleted) return file.oldName;
	return file.newName;
}

/**
 * Detect the language from a filename for syntax highlighting.
 */
export function detectLanguage(filename: string): string {
	const ext = filename.split('.').pop()?.toLowerCase() ?? '';
	const langMap: Record<string, string> = {
		go: 'go',
		ts: 'typescript',
		tsx: 'typescript',
		js: 'javascript',
		jsx: 'javascript',
		json: 'json',
		yaml: 'yaml',
		yml: 'yaml',
		md: 'markdown',
		html: 'html',
		svelte: 'html',
		css: 'css',
		py: 'python',
		sh: 'bash',
		bash: 'bash',
		sql: 'sql',
		toml: 'toml',
		rs: 'rust',
		mod: 'go',
		sum: 'text'
	};
	return langMap[ext] ?? 'text';
}
