// @vitest-environment jsdom
import { describe, it, expect, beforeEach } from 'vitest';
import { applyInlineAnnotations, type InlineMark } from './inline-annotations';

function el(tag: string, opts: { dataSourceLine?: number; text?: string } = {}): HTMLElement {
	const e = document.createElement(tag);
	if (opts.dataSourceLine !== undefined) {
		e.setAttribute('data-source-line', String(opts.dataSourceLine));
	}
	if (opts.text !== undefined) e.appendChild(document.createTextNode(opts.text));
	return e;
}

function makeContainer(): HTMLElement {
	return document.createElement('article');
}

function mark(opts: Partial<InlineMark> & Pick<InlineMark, 'quotedText' | 'lineStart' | 'lineEnd'>): InlineMark {
	return {
		id: opts.id ?? 'm1',
		kind: opts.kind ?? 'comment',
		lineStart: opts.lineStart,
		lineEnd: opts.lineEnd,
		quotedText: opts.quotedText
	};
}

describe('applyInlineAnnotations', () => {
	let container: HTMLElement;
	beforeEach(() => {
		container = makeContainer();
	});

	it('wraps a single contiguous selection inside one paragraph', () => {
		const p = el('p', { dataSourceLine: 1, text: 'The quick brown fox' });
		container.appendChild(p);

		applyInlineAnnotations(container, [
			mark({ quotedText: 'quick brown', lineStart: 1, lineEnd: 1 })
		]);

		const marks = container.querySelectorAll('mark.anno-comment');
		expect(marks.length).toBe(1);
		expect(marks[0].textContent).toBe('quick brown');
		expect(marks[0].parentElement?.tagName).toBe('P');
	});

	it('wraps text inside an inline element (e.g. <code>) without splitting structure', () => {
		const p = el('p', { dataSourceLine: 1 });
		p.appendChild(document.createTextNode('use '));
		const code = el('code', { text: 'foo()' });
		p.appendChild(code);
		p.appendChild(document.createTextNode(' here'));
		container.appendChild(p);

		applyInlineAnnotations(container, [
			mark({ quotedText: 'use foo() here', lineStart: 1, lineEnd: 1 })
		]);

		// The whole phrase should be wrapped — possibly as a single Range surround
		// (one <mark>) or as per-text-node wraps (three <mark>s). Either is OK
		// as long as the visible covered text is exactly the needle.
		const marks = Array.from(container.querySelectorAll('mark.anno-comment'));
		expect(marks.length).toBeGreaterThanOrEqual(1);
		const covered = marks.map((m) => m.textContent ?? '').join('');
		expect(covered).toBe('use foo() here');
		// Crucially, <code> is still present and still contains its text.
		expect(container.querySelector('code')?.textContent).toBe('foo()');
	});

	it('wraps a multi-paragraph selection where the needle has \\n separators', () => {
		const p1 = el('p', { dataSourceLine: 1, text: 'First paragraph.' });
		const p2 = el('p', { dataSourceLine: 3, text: 'Second paragraph.' });
		container.appendChild(p1);
		container.appendChild(p2);

		applyInlineAnnotations(container, [
			mark({ quotedText: 'First paragraph.\nSecond paragraph.', lineStart: 1, lineEnd: 3 })
		]);

		const marks = Array.from(container.querySelectorAll('mark.anno-comment'));
		expect(marks.length).toBeGreaterThanOrEqual(2);
		expect(marks.some((m) => m.textContent === 'First paragraph.')).toBe(true);
		expect(marks.some((m) => m.textContent === 'Second paragraph.')).toBe(true);
		// Paragraph elements remain intact.
		expect(container.querySelectorAll('p').length).toBe(2);
	});

	it('wraps a heading + bulleted list selection (the regression case)', () => {
		// Reproduces the "Non-goals" failure from the handoff: <h2> followed by
		// a <ul> with several <li>s. The TreeWalker yields text nodes with no
		// whitespace between them, but selection.toString() inserts \n between
		// the heading and each <li> — so the search needs synthetic block
		// separators to match.
		const h = el('h2', { dataSourceLine: 1, text: 'Non-goals' });
		const ul = el('ul', { dataSourceLine: 3 });
		const li1 = el('li', { text: 'Real authentication, OAuth, or accounts' });
		const li2 = el('li', { text: 'Server-side comment aggregation' });
		const li3 = el('li', { text: 'Live multi-user co-editing' });
		ul.appendChild(li1);
		ul.appendChild(li2);
		ul.appendChild(li3);
		container.appendChild(h);
		container.appendChild(ul);

		const needle = [
			'Non-goals',
			'Real authentication, OAuth, or accounts',
			'Server-side comment aggregation',
			'Live multi-user co-editing'
		].join('\n');

		applyInlineAnnotations(container, [mark({ quotedText: needle, lineStart: 1, lineEnd: 3 })]);

		const marks = Array.from(container.querySelectorAll('mark.anno-comment'));
		// One mark per text node (heading + 3 list items).
		expect(marks.length).toBe(4);
		expect(marks.map((m) => m.textContent ?? '')).toEqual([
			'Non-goals',
			'Real authentication, OAuth, or accounts',
			'Server-side comment aggregation',
			'Live multi-user co-editing'
		]);
		// List structure is preserved — three <li> children of one <ul>.
		expect(container.querySelectorAll('ul > li').length).toBe(3);
	});

	it('wraps a code-block selection containing literal newlines', () => {
		// <pre><code>...</code></pre> — text contains real \n chars, not block
		// boundaries. The needle from selection.toString() also has real \n.
		const pre = el('pre', { dataSourceLine: 1 });
		const code = el('code');
		code.appendChild(document.createTextNode('line one\nline two\nline three'));
		pre.appendChild(code);
		container.appendChild(pre);

		applyInlineAnnotations(container, [
			mark({ quotedText: 'line one\nline two\nline three', lineStart: 1, lineEnd: 1 })
		]);

		const marks = Array.from(container.querySelectorAll('mark.anno-comment'));
		expect(marks.length).toBeGreaterThanOrEqual(1);
		const covered = marks.map((m) => m.textContent ?? '').join('');
		expect(covered).toBe('line one\nline two\nline three');
		// <pre><code> structure preserved.
		expect(container.querySelector('pre > code')).toBeTruthy();
	});

	it('treats <br> as a line break (the markdown hard-break case)', () => {
		// Markdown's two-trailing-spaces hard break renders as <br> inside a
		// single <li> or <p>. Selection.toString() emits '\n' at the <br>; the
		// TreeWalker SHOW_TEXT path doesn't see it. Without explicit handling
		// the search whiffs because needle has '\n' but searchSpace doesn't.
		const ul = el('ul', { dataSourceLine: 1 });
		const li = el('li');
		li.appendChild(document.createTextNode('they remain as'));
		li.appendChild(document.createElement('br'));
		li.appendChild(document.createTextNode('legacy storage'));
		ul.appendChild(li);
		container.appendChild(ul);

		applyInlineAnnotations(container, [
			mark({ quotedText: 'they remain as\nlegacy storage', lineStart: 1, lineEnd: 1 })
		]);

		const marks = Array.from(container.querySelectorAll('mark.anno-comment'));
		// Either a single Range wrap (1 mark covering text+<br>+text) or per-
		// text-node fallback (2 marks). Both are correct outcomes; what matters
		// is that *something* wrapped, the structural <br> survived, and both
		// halves of the text are inside a mark.
		expect(marks.length).toBeGreaterThanOrEqual(1);
		const covered = marks.map((m) => m.textContent ?? '').join('|');
		expect(covered).toContain('they remain as');
		expect(covered).toContain('legacy storage');
		expect(container.querySelectorAll('br').length).toBe(1);
	});

	it('handles a partial selection inside a single <li>', () => {
		const ul = el('ul', { dataSourceLine: 1 });
		ul.appendChild(el('li', { text: 'one apple' }));
		ul.appendChild(el('li', { text: 'two oranges' }));
		container.appendChild(ul);

		applyInlineAnnotations(container, [
			mark({ quotedText: 'two oranges', lineStart: 1, lineEnd: 1 })
		]);

		const marks = Array.from(container.querySelectorAll('mark.anno-comment'));
		expect(marks.length).toBe(1);
		expect(marks[0].textContent).toBe('two oranges');
		expect(container.querySelectorAll('ul > li').length).toBe(2);
	});

	it('clears prior marks on re-application', () => {
		const p = el('p', { dataSourceLine: 1, text: 'Hello world' });
		container.appendChild(p);

		const m1: InlineMark = mark({ quotedText: 'Hello', lineStart: 1, lineEnd: 1, id: 'a' });
		const m2: InlineMark = mark({ quotedText: 'world', lineStart: 1, lineEnd: 1, id: 'b' });

		applyInlineAnnotations(container, [m1]);
		expect(container.querySelectorAll('mark[data-anno-id="a"]').length).toBe(1);

		applyInlineAnnotations(container, [m2]);
		// First mark torn down, second applied.
		expect(container.querySelectorAll('mark[data-anno-id="a"]').length).toBe(0);
		expect(container.querySelectorAll('mark[data-anno-id="b"]').length).toBe(1);
	});

	it('returns silently when the anchor is missing', () => {
		const p = el('p', { dataSourceLine: 1, text: 'Hello' });
		container.appendChild(p);

		applyInlineAnnotations(container, [
			mark({ quotedText: 'goodbye', lineStart: 1, lineEnd: 1 })
		]);

		expect(container.querySelectorAll('mark.anno-comment').length).toBe(0);
		// Original text untouched.
		expect(p.textContent).toBe('Hello');
	});
});
