import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { isMacLike, modifierGlyph } from './platform';

// Snapshot the original navigator so each test starts from a clean baseline.
// We replace it wholesale (rather than mutate) because `navigator.platform`
// is read-only in some environments.
const originalNavigator = globalThis.navigator;

// `Navigator.platform` is typed as a narrow literal union by TypeScript's
// DOM lib, but the runtime value is a free-form string and our helper does a
// case-insensitive substring check. Use a loose record so tests can stub
// arbitrary platform strings (e.g. "macppc", "Linux x86_64") without
// fighting the type system.
function setNavigator(stub: { platform?: string; userAgentData?: { platform?: string } }) {
	Object.defineProperty(globalThis, 'navigator', {
		value: stub,
		writable: true,
		configurable: true
	});
}

describe('isMacLike', () => {
	beforeEach(() => {
		// Default: an empty navigator so individual tests opt into a platform.
		setNavigator({ platform: '' });
	});

	afterEach(() => {
		Object.defineProperty(globalThis, 'navigator', {
			value: originalNavigator,
			writable: true,
			configurable: true
		});
	});

	it('returns true when userAgentData.platform is "macOS"', () => {
		setNavigator({ platform: 'unused', userAgentData: { platform: 'macOS' } });
		expect(isMacLike()).toBe(true);
	});

	it('returns false when userAgentData.platform is non-macOS', () => {
		setNavigator({ platform: 'MacIntel', userAgentData: { platform: 'Linux' } });
		// Modern UA-Client-Hints take priority — even if the deprecated
		// `platform` string lies, we trust the new API first.
		expect(isMacLike()).toBe(false);
	});

	it('falls back to navigator.platform when userAgentData is absent', () => {
		setNavigator({ platform: 'MacIntel' });
		expect(isMacLike()).toBe(true);
	});

	it('matches case-insensitively in the legacy fallback', () => {
		setNavigator({ platform: 'macppc' });
		expect(isMacLike()).toBe(true);
	});

	it('returns false for Linux', () => {
		setNavigator({ platform: 'Linux x86_64' });
		expect(isMacLike()).toBe(false);
	});

	it('returns false for Windows', () => {
		setNavigator({ platform: 'Win32' });
		expect(isMacLike()).toBe(false);
	});
});

describe('modifierGlyph', () => {
	beforeEach(() => {
		setNavigator({ platform: '' });
	});

	afterEach(() => {
		Object.defineProperty(globalThis, 'navigator', {
			value: originalNavigator,
			writable: true,
			configurable: true
		});
	});

	it('returns ⌘ on macOS', () => {
		setNavigator({ platform: 'MacIntel' });
		expect(modifierGlyph()).toBe('⌘');
	});

	it('returns Ctrl elsewhere', () => {
		setNavigator({ platform: 'Linux' });
		expect(modifierGlyph()).toBe('Ctrl');
	});
});
