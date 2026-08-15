// @vitest-environment jsdom
import { describe, it, expect, beforeEach } from 'vitest';
import { applyInlineAnnotations, blockSearchText, clearMarks } from './inline-annotations';
import type { InlineMark } from './types';

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

function mark(
	opts: Partial<InlineMark> & Pick<InlineMark, 'quotedText' | 'lineStart' | 'lineEnd'>
): InlineMark {
	return {
		id: opts.id ?? 'm1',
		quotedText: opts.quotedText,
		occurrence: opts.occurrence ?? 0,
		lineStart: opts.lineStart,
		lineEnd: opts.lineEnd,
		resolved: opts.resolved ?? false,
		drifted: opts.drifted ?? false
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

		applyInlineAnnotations(container, [mark({ quotedText: 'two oranges', lineStart: 1, lineEnd: 1 })]);

		const marks = Array.from(container.querySelectorAll('mark.anno-comment'));
		expect(marks.length).toBe(1);
		expect(marks[0].textContent).toBe('two oranges');
		expect(container.querySelectorAll('ul > li').length).toBe(2);
	});

	it('clears prior marks on re-application', () => {
		const p = el('p', { dataSourceLine: 1, text: 'Hello world' });
		container.appendChild(p);

		const m1 = mark({ quotedText: 'Hello', lineStart: 1, lineEnd: 1, id: 'a' });
		const m2 = mark({ quotedText: 'world', lineStart: 1, lineEnd: 1, id: 'b' });

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

		applyInlineAnnotations(container, [mark({ quotedText: 'goodbye', lineStart: 1, lineEnd: 1 })]);

		expect(container.querySelectorAll('mark.anno-comment').length).toBe(0);
		// Original text untouched.
		expect(p.textContent).toBe('Hello');
	});

	it('wraps the nth occurrence when the needle repeats inside one block', () => {
		const p = el('p', { dataSourceLine: 1, text: 'alpha beta alpha gamma' });
		container.appendChild(p);

		applyInlineAnnotations(container, [
			mark({ quotedText: 'alpha', lineStart: 1, lineEnd: 1, occurrence: 1 })
		]);

		const marks = Array.from(container.querySelectorAll('mark.anno-comment'));
		expect(marks.length).toBe(1);
		expect(marks[0].textContent).toBe('alpha');
		// The SECOND instance: everything before it is still bare text.
		expect(marks[0].previousSibling?.textContent).toBe('alpha beta ');
		expect(marks[0].nextSibling?.textContent).toBe(' gamma');
	});

	it('recomputes a doc-wide occurrence into the narrowed block range', () => {
		const p1 = el('p', { dataSourceLine: 3, text: 'the same text here' });
		const p2 = el('p', { dataSourceLine: 9, text: 'the same text here' });
		container.appendChild(p1);
		container.appendChild(p2);

		applyInlineAnnotations(container, [
			mark({ quotedText: 'the same text here', lineStart: 9, lineEnd: 9, occurrence: 1 })
		]);

		const marks = Array.from(container.querySelectorAll('mark.anno-comment'));
		expect(marks.length).toBe(1);
		expect(p1.querySelector('mark')).toBeNull();
		expect(p2.querySelector('mark')?.textContent).toBe('the same text here');
	});

	it('recomputes the normalized-tier occurrence in normalized space, not raw space', () => {
		// Three blocks, all normalizing to "run task build" but each with
		// different raw whitespace so the exact tier can never match:
		//   - line 1 (outside the mark's range): triple space — an earlier
		//     occurrence that DOES count toward the normalized "before" total
		//     (it normalizes to the needle) but does NOT count toward the raw
		//     "before" total (its raw text never equals the needle exactly).
		//     That's exactly the gap the fix has to account for.
		//   - line 3 and line 9 (the mark's narrowed range): double space and
		//     single space, respectively.
		// The mark's quotedText uses a tab, which is whitespace-equivalent to
		// a single space after normalization but byte-for-byte different from
		// all three blocks' raw text — so the exact tier misses everywhere,
		// forcing every block into the whitespace-normalized fallback tier.
		const decoy = el('p', { dataSourceLine: 1, text: 'run   task build' });
		const p1 = el('p', { dataSourceLine: 3, text: 'run  task build' });
		const p2 = el('p', { dataSourceLine: 9, text: 'run task build' });
		container.appendChild(decoy);
		container.appendChild(p1);
		container.appendChild(p2);

		applyInlineAnnotations(container, [
			mark({ quotedText: 'run\ttask build', lineStart: 3, lineEnd: 9, occurrence: 1 })
		]);

		const marks = Array.from(container.querySelectorAll('mark.anno-comment'));
		expect(marks.length).toBe(1);
		// The doc-wide occurrence (1) minus the one earlier normalized-only
		// match (the decoy) lands on occurrence 0 within the narrowed range:
		// the block at line 3, not line 9. Indexing normMatches with the raw
		// "before" count (which doesn't discount the decoy at all, since it
		// never raw-matches) would instead wrap line 9.
		expect(decoy.querySelector('mark')).toBeNull();
		expect(p1.querySelector('mark')?.textContent).toBe('run  task build');
		expect(p2.querySelector('mark')).toBeNull();
	});

	it('clamps a stale occurrence to the last match', () => {
		const p = el('p', { dataSourceLine: 1, text: 'x foo y foo z' });
		container.appendChild(p);

		applyInlineAnnotations(container, [
			mark({ quotedText: 'foo', lineStart: 1, lineEnd: 1, occurrence: 4 })
		]);

		const marks = Array.from(container.querySelectorAll('mark.anno-comment'));
		expect(marks.length).toBe(1);
		expect(marks[0].textContent).toBe('foo');
		expect(marks[0].previousSibling?.textContent).toBe('x foo y ');
	});

	it('snaps a drifted lineStart back to the enclosing tagged block', () => {
		const p1 = el('p', { dataSourceLine: 30, text: 'Alpha block content' });
		const p2 = el('p', { dataSourceLine: 40, text: 'Beta block content' });
		container.appendChild(p1);
		container.appendChild(p2);

		// lineStart 35 lands between the tagged blocks (30 and 40).
		applyInlineAnnotations(container, [
			mark({ quotedText: 'Alpha block', lineStart: 35, lineEnd: 35 })
		]);

		const marks = Array.from(container.querySelectorAll('mark.anno-comment'));
		expect(marks.length).toBe(1);
		expect(marks[0].textContent).toBe('Alpha block');
		expect(marks[0].parentElement).toBe(p1);
	});

	it('snaps forward when lineStart precedes every tagged block', () => {
		const p = el('p', { dataSourceLine: 10, text: 'Alpha block content' });
		container.appendChild(p);

		applyInlineAnnotations(container, [
			mark({ quotedText: 'Alpha block', lineStart: 4, lineEnd: 4 })
		]);

		const marks = Array.from(container.querySelectorAll('mark.anno-comment'));
		expect(marks.length).toBe(1);
		expect(marks[0].parentElement).toBe(p);
	});

	it('applies resolved / drifted / active state classes', () => {
		const p1 = el('p', { dataSourceLine: 1, text: 'one apple' });
		const p2 = el('p', { dataSourceLine: 2, text: 'two oranges' });
		const p3 = el('p', { dataSourceLine: 3, text: 'three pears' });
		container.appendChild(p1);
		container.appendChild(p2);
		container.appendChild(p3);

		applyInlineAnnotations(
			container,
			[
				mark({ id: 'a', quotedText: 'apple', lineStart: 1, lineEnd: 1, resolved: true }),
				mark({ id: 'b', quotedText: 'oranges', lineStart: 2, lineEnd: 2, drifted: true }),
				mark({ id: 'c', quotedText: 'pears', lineStart: 3, lineEnd: 3 })
			],
			'c'
		);

		const a = container.querySelector<HTMLElement>('mark[data-anno-id="a"]');
		const b = container.querySelector<HTMLElement>('mark[data-anno-id="b"]');
		const c = container.querySelector<HTMLElement>('mark[data-anno-id="c"]');
		expect(a?.classList.contains('anno-comment')).toBe(true);
		expect(a?.classList.contains('is-resolved')).toBe(true);
		expect(a?.classList.contains('is-drifted')).toBe(false);
		expect(a?.classList.contains('is-active')).toBe(false);
		expect(b?.classList.contains('is-drifted')).toBe(true);
		expect(b?.classList.contains('is-resolved')).toBe(false);
		expect(c?.classList.contains('is-active')).toBe(true);
	});
});

describe('clearMarks', () => {
	it('unwraps marks and merges the surrounding text back together', () => {
		const container = makeContainer();
		const p = el('p', { dataSourceLine: 1, text: 'Hello world' });
		container.appendChild(p);

		applyInlineAnnotations(container, [
			mark({ quotedText: 'lo wo', lineStart: 1, lineEnd: 1, id: 'a' })
		]);
		expect(container.querySelectorAll('mark[data-anno-id]').length).toBe(1);

		clearMarks(container);

		expect(container.querySelectorAll('mark[data-anno-id]').length).toBe(0);
		expect(p.textContent).toBe('Hello world');
		// normalize() merged the split text nodes back into one.
		expect(p.childNodes.length).toBe(1);
	});
});

describe('blockSearchText', () => {
	it('joins list items with a newline', () => {
		const ul = el('ul', { dataSourceLine: 1 });
		ul.appendChild(el('li', { text: 'first item' }));
		ul.appendChild(el('li', { text: 'second item' }));

		expect(blockSearchText(ul)).toBe('first item\nsecond item');
	});

	it('treats <br> as a newline', () => {
		const p = el('p', { dataSourceLine: 1 });
		p.appendChild(document.createTextNode('a'));
		p.appendChild(document.createElement('br'));
		p.appendChild(document.createTextNode('b'));

		expect(blockSearchText(p)).toBe('a\nb');
	});

	it('returns the raw text for a plain paragraph', () => {
		const p = el('p', { dataSourceLine: 1, text: 'The quick brown fox' });
		expect(blockSearchText(p)).toBe('The quick brown fox');
	});
});
