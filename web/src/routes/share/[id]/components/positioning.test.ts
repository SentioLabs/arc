import { describe, it, expect } from 'vitest';
import { clampedAnchorLeft } from './positioning';

describe('clampedAnchorLeft', () => {
	it('returns the raw center when the overlay fits without clipping', () => {
		// 360px overlay at center 600 in a 1200px viewport: extends 420..780,
		// clear of both 8px margins.
		expect(clampedAnchorLeft(600, 360, 1200)).toBe(600);
	});

	it('pushes overlay rightward when selection is too close to the left edge', () => {
		// 360px overlay anchored at center 50 would extend -130..230, clipping
		// past the left edge. Clamp must push the center to 8 + 180 = 188.
		expect(clampedAnchorLeft(50, 360, 1200)).toBe(188);
	});

	it('pushes overlay leftward when selection is too close to the right edge', () => {
		// 360px overlay anchored at center 1180 would extend 1000..1360,
		// clipping past the right edge. Clamp must pull center to 1200 - 8 - 180 = 1012.
		expect(clampedAnchorLeft(1180, 360, 1200)).toBe(1012);
	});

	it('respects a custom margin', () => {
		// With a 20px margin, the left clamp is 20 + 180 = 200.
		expect(clampedAnchorLeft(50, 360, 1200, 20)).toBe(200);
	});

	it('falls back to pinning when overlay is wider than viewport', () => {
		// 800px overlay in a 600px viewport can never satisfy both bounds.
		// Behavior: pin to left margin (start of controls visible).
		expect(clampedAnchorLeft(300, 800, 600)).toBe(8 + 400);
	});

	it('handles selection exactly at the left clamp threshold', () => {
		// 360px overlay, viewport 1200, margin 8 → minLeft = 188. At 188 no clamp needed.
		expect(clampedAnchorLeft(188, 360, 1200)).toBe(188);
	});
});
