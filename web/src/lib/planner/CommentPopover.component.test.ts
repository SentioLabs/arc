// @vitest-environment jsdom
import { afterEach, expect, test, vi } from 'vitest';
import { mount, unmount, flushSync, tick } from 'svelte';
import CommentPopover from './CommentPopover.svelte';

let component: ReturnType<typeof mount>;
afterEach(async () => {
	if (component) await unmount(component);
	document.body.innerHTML = '';
	vi.restoreAllMocks();
});
async function render(onSave = vi.fn(), onCancel = vi.fn()) {
	component = mount(CommentPopover, {
		target: document.body,
		props: {
			anchorRect: new DOMRect(200, 200, 100, 30),
			quotedText: 'Selected phrase',
			onSave,
			onCancel
		}
	});
	flushSync();
	await tick();
	return { onSave, onCancel };
}
function type(value: string) {
	const input = document.querySelector('textarea') as HTMLTextAreaElement;
	input.value = value;
	input.dispatchEvent(new Event('input', { bubbles: true }));
	flushSync();
}
test('dirty draft survives scroll and rejected escape, then confirms cancellation', async () => {
	const { onCancel } = await render();
	const confirm = vi.spyOn(window, 'confirm').mockReturnValue(false);
	type('unsaved');
	window.dispatchEvent(new Event('scroll'));
	expect(onCancel).not.toHaveBeenCalled();
	expect(confirm).not.toHaveBeenCalled();
	document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
	expect(confirm).toHaveBeenCalledOnce();
	expect(onCancel).not.toHaveBeenCalled();
	confirm.mockReturnValue(true);
	document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
	expect(onCancel).toHaveBeenCalledOnce();
});
test('blank save disabled; async keyboard save trims and prevents duplicate submissions', async () => {
	let resolveSave!: () => void;
	const onSave = vi.fn(
		() =>
			new Promise<void>((resolve) => {
				resolveSave = resolve;
			})
	);
	await render(onSave);
	const save = Array.from(document.querySelectorAll('button')).find(
		(button) => button.textContent?.trim() === 'Save'
	);
	if (!save) throw new Error('Save button missing');
	expect(save.disabled).toBe(true);
	type('  useful feedback  ');
	expect(save.disabled).toBe(false);
	document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', ctrlKey: true }));
	flushSync();
	expect(save.disabled).toBe(true);
	document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', ctrlKey: true }));
	expect(onSave).toHaveBeenCalledExactlyOnceWith('useful feedback');
	resolveSave();
	await tick();
	flushSync();
	expect(save.disabled).toBe(false);
});
test('clean scroll dismisses and unmount removes document listeners', async () => {
	const { onCancel } = await render();
	window.dispatchEvent(new Event('scroll'));
	expect(onCancel).toHaveBeenCalledOnce();
	await unmount(component);
	document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
	expect(onCancel).toHaveBeenCalledOnce();
});
