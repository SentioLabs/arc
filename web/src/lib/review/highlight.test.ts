import { describe, it, expect } from 'bun:test';
import { highlightDiffFile, getLineContent } from './highlight';
import { LineType, type DiffFile } from 'diff2html/lib/types';

function makeDiffFile(overrides: Partial<DiffFile> = {}): DiffFile {
	return {
		oldName: 'example.ts',
		newName: 'example.ts',
		addedLines: 0,
		deletedLines: 0,
		isCombined: false,
		isGitDiff: true,
		language: 'typescript',
		blocks: [],
		...overrides
	};
}

describe('getLineContent', () => {
	it('strips the leading diff prefix character from a line', () => {
		expect(getLineContent({ content: '+const x = 1;' })).toBe('const x = 1;');
		expect(getLineContent({ content: '-const y = 2;' })).toBe('const y = 2;');
		expect(getLineContent({ content: ' const z = 3;' })).toBe('const z = 3;');
	});

	it('handles empty content after prefix', () => {
		expect(getLineContent({ content: '+' })).toBe('');
		expect(getLineContent({ content: ' ' })).toBe('');
	});
});

describe('highlightDiffFile', () => {
	it('returns an empty map for a file with no blocks', async () => {
		const file = makeDiffFile({ blocks: [] });
		const result = await highlightDiffFile(file);
		expect(result).toBeInstanceOf(Map);
		expect(result.size).toBe(0);
	});

	it('returns highlighted HTML for each unique line content', async () => {
		const file = makeDiffFile({
			newName: 'example.ts',
			blocks: [
				{
					oldStartLine: 1,
					newStartLine: 1,
					header: '@@ -1,2 +1,3 @@',
					lines: [
						{
							type: LineType.CONTEXT,
							oldNumber: 1,
							newNumber: 1,
							content: ' const a = 1;'
						},
						{
							type: LineType.DELETE,
							oldNumber: 2,
							newNumber: undefined,
							content: '-const b = 2;'
						},
						{
							type: LineType.INSERT,
							oldNumber: undefined,
							newNumber: 2,
							content: '+const b = 3;'
						},
						{
							type: LineType.INSERT,
							oldNumber: undefined,
							newNumber: 3,
							content: '+const c = 4;'
						}
					]
				}
			]
		});

		const result = await highlightDiffFile(file);

		// Should have entries for each unique stripped line
		expect(result.has('const a = 1;')).toBe(true);
		expect(result.has('const b = 2;')).toBe(true);
		expect(result.has('const b = 3;')).toBe(true);
		expect(result.has('const c = 4;')).toBe(true);

		// Each value should be an HTML string (Shiki wraps tokens in <span>)
		for (const html of result.values()) {
			expect(typeof html).toBe('string');
			expect(html.length).toBeGreaterThan(0);
			expect(html).toContain('<span');
		}
	});

	it('deduplicates identical line contents', async () => {
		const file = makeDiffFile({
			newName: 'example.ts',
			blocks: [
				{
					oldStartLine: 1,
					newStartLine: 1,
					header: '@@ -1,2 +1,2 @@',
					lines: [
						{
							type: LineType.DELETE,
							oldNumber: 1,
							newNumber: undefined,
							content: '-const x = 1;'
						},
						{
							type: LineType.INSERT,
							oldNumber: undefined,
							newNumber: 1,
							content: '+const x = 1;'
						}
					]
				}
			]
		});

		const result = await highlightDiffFile(file);

		// Both lines have the same stripped content, so only one entry
		expect(result.size).toBe(1);
		expect(result.has('const x = 1;')).toBe(true);
	});

	it('detects language from filename for highlighting', async () => {
		const goFile = makeDiffFile({
			oldName: 'main.go',
			newName: 'main.go',
			blocks: [
				{
					oldStartLine: 1,
					newStartLine: 1,
					header: '@@ -1,1 +1,1 @@',
					lines: [
						{
							type: LineType.INSERT,
							oldNumber: undefined,
							newNumber: 1,
							content: '+func main() {'
						}
					]
				}
			]
		});

		const result = await highlightDiffFile(goFile);
		expect(result.has('func main() {')).toBe(true);
		// The highlighted HTML should contain span tags from Shiki
		const html = result.get('func main() {')!;
		expect(html).toContain('<span');
	});
});
