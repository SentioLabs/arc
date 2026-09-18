// @vitest-environment jsdom
import { afterEach, describe, expect, test } from 'vitest';
import { mount, unmount, flushSync } from 'svelte';
import TranscriptViewer from './TranscriptViewer.svelte';

let component: ReturnType<typeof mount>;
afterEach(async () => {
	if (component) await unmount(component);
	document.body.innerHTML = '';
});
function render(transcript: Record<string, unknown>[]) {
	component = mount(TranscriptViewer, { target: document.body, props: { transcript } });
	flushSync();
}
describe('TranscriptViewer rendered behavior', () => {
	test('handles empty transcript', () => {
		render([]);
		expect(document.body.textContent).toContain('No transcript data available');
	});
	test.each([
		['user', 'User', 'border-l-blue-400', 'text-blue-400'],
		['assistant', 'Assistant', 'border-l-primary-400', 'text-primary-400'],
		['system', 'System', 'border-l-text-muted', 'text-text-muted']
	])('renders %s messages with distinct role styling', (role, label, border, color) => {
		render([{ role, content: 'Message body' }]);
		const row = document.querySelector('.border-l-2');
		expect(row?.classList.contains(border)).toBe(true);
		expect(row?.firstElementChild?.textContent).toBe(label);
		expect(row?.firstElementChild?.classList.contains(color)).toBe(true);
		expect(row?.textContent).toContain('Message body');
	});
	test('tool headers expand formatted JSON and collapse again', () => {
		render([
			{ type: 'tool_use', name: 'Read', input: { path: '/tmp/example' }, content: 'result' }
		]);
		const button = document.querySelector('button');
		expect(button?.textContent).toContain('Read');
		expect(document.querySelectorAll('pre')).toHaveLength(0);
		button?.click();
		flushSync();
		expect(document.querySelector('pre')?.textContent).toBe(
			JSON.stringify({ path: '/tmp/example' }, null, 2)
		);
		expect(document.body.textContent).toContain('result');
		button?.click();
		flushSync();
		expect(document.querySelectorAll('pre')).toHaveLength(0);
	});
	test('inline tools remain interactive alongside assistant text', () => {
		render([
			{
				role: 'assistant',
				content: [
					{ type: 'text', text: 'Inspecting file' },
					{ type: 'tool_use', name: 'Read', input: { path: 'file' } }
				]
			}
		]);
		expect(document.body.textContent).toContain('Inspecting file');
		document.querySelector('button')?.click();
		flushSync();
		expect(document.querySelector('pre')?.textContent).toContain('file');
	});
});
