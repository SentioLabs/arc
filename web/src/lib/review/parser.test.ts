import { describe, expect, test } from 'bun:test';
import { parseDiff, getFileName, detectLanguage } from './parser';

const SAMPLE_DIFF = `diff --git a/hello.go b/hello.go
new file mode 100644
index 0000000..1234567
--- /dev/null
+++ b/hello.go
@@ -0,0 +1,5 @@
+package main
+
+func main() {
+    println("hello")
+}
`;

const MULTI_FILE_DIFF = `diff --git a/a.ts b/a.ts
index 1111111..2222222 100644
--- a/a.ts
+++ b/a.ts
@@ -1,3 +1,4 @@
 const x = 1;
-const y = 2;
+const y = 3;
+const z = 4;
 export { x };
diff --git a/b.ts b/b.ts
deleted file mode 100644
index 3333333..0000000
--- a/b.ts
+++ /dev/null
@@ -1,2 +0,0 @@
-const old = true;
-export { old };
`;

describe('parseDiff', () => {
    test('parses a single new file diff', () => {
        const result = parseDiff(SAMPLE_DIFF);
        expect(result.files).toHaveLength(1);
        expect(result.stats.filesChanged).toBe(1);
        expect(result.stats.totalAdditions).toBe(5);
        expect(result.stats.totalDeletions).toBe(0);
    });

    test('parses a multi-file diff with additions and deletions', () => {
        const result = parseDiff(MULTI_FILE_DIFF);
        expect(result.files).toHaveLength(2);
        expect(result.stats.filesChanged).toBe(2);
        // a.ts: +2 -1, b.ts: +0 -2
        expect(result.stats.totalAdditions).toBe(2);
        expect(result.stats.totalDeletions).toBe(3);
    });

    test('returns empty files array for empty string', () => {
        const result = parseDiff('');
        expect(result.files).toHaveLength(0);
        expect(result.stats.filesChanged).toBe(0);
        expect(result.stats.totalAdditions).toBe(0);
        expect(result.stats.totalDeletions).toBe(0);
    });

    test('files contain blocks with lines', () => {
        const result = parseDiff(SAMPLE_DIFF);
        const file = result.files[0];
        expect(file.blocks.length).toBeGreaterThan(0);
        expect(file.blocks[0].lines.length).toBeGreaterThan(0);
    });
});

describe('getFileName', () => {
    test('returns newName for normal files', () => {
        const result = parseDiff(MULTI_FILE_DIFF);
        const modifiedFile = result.files[0];
        expect(getFileName(modifiedFile)).toBe('a.ts');
    });

    test('returns oldName for deleted files', () => {
        const result = parseDiff(MULTI_FILE_DIFF);
        const deletedFile = result.files[1];
        expect(getFileName(deletedFile)).toBe('b.ts');
    });
});

describe('detectLanguage', () => {
    test('maps Go files', () => {
        expect(detectLanguage('main.go')).toBe('go');
    });

    test('maps TypeScript files', () => {
        expect(detectLanguage('index.ts')).toBe('typescript');
        expect(detectLanguage('App.tsx')).toBe('typescript');
    });

    test('maps JavaScript files', () => {
        expect(detectLanguage('index.js')).toBe('javascript');
    });

    test('maps Svelte files to html', () => {
        expect(detectLanguage('Component.svelte')).toBe('html');
    });

    test('maps Python files', () => {
        expect(detectLanguage('script.py')).toBe('python');
    });

    test('maps shell files', () => {
        expect(detectLanguage('run.sh')).toBe('bash');
    });

    test('maps go.mod to go', () => {
        expect(detectLanguage('go.mod')).toBe('go');
    });

    test('returns text for unknown extensions', () => {
        expect(detectLanguage('file.xyz')).toBe('text');
    });

    test('returns text for files with no extension', () => {
        expect(detectLanguage('Makefile')).toBe('text');
    });
});
