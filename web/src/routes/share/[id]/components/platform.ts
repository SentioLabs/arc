/**
 * Platform detection for displaying the right keyboard-shortcut glyph.
 *
 * Mac users expect ⌘ ⏎; everyone else expects Ctrl ⏎. The keyboard
 * handlers already accept both `metaKey` and `ctrlKey`, so this only
 * affects the on-screen hint.
 *
 * Detection strategy:
 *   1. `navigator.userAgentData.platform` — modern, available in Chromium.
 *   2. `navigator.platform` — deprecated but still populated by every
 *      browser. We use `.toLowerCase().includes('mac')` to also catch
 *      the older "MacIntel" / "MacPPC" values.
 *   3. SSR fallback — return false. The first client paint will correct
 *      itself; SvelteKit's `+page.svelte` renders client-side after the
 *      paste blob decrypts anyway, so we never actually hit SSR for the
 *      share UI.
 */
export function isMacLike(): boolean {
	if (typeof navigator === 'undefined') return false;
	type UAData = { platform?: string };
	const uaData = (navigator as Navigator & { userAgentData?: UAData }).userAgentData;
	if (uaData?.platform) {
		return uaData.platform === 'macOS';
	}
	return navigator.platform.toLowerCase().includes('mac');
}

/**
 * Glyph shown for the modifier in `<modifier> ⏎` keyboard hints.
 * `⌘` on macOS, `Ctrl` everywhere else.
 */
export function modifierGlyph(): string {
	return isMacLike() ? '⌘' : 'Ctrl';
}
