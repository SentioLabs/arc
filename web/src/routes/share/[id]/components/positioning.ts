/**
 * Positioning helpers for the floating overlays (toolbar, popover, picker).
 *
 * Each overlay anchors above the user's selection. The naive computation —
 * `rect.left + rect.width/2` paired with `transform: translateX(-50%)` —
 * works in isolation but clips the overlay off-screen when the selection
 * is near the viewport's left or right edge. With the share page's
 * asymmetric inset (doc sits ~72px from the left), a comment popover
 * (360px wide) anchored to a selection at the start of the prose column
 * extends ~108px past the left edge of the viewport.
 *
 * `clampedAnchorLeft` returns the `left` value to plug into the overlay's
 * inline style. The caller still applies `translateX(-50%)`. After clamping,
 * the overlay's edges are guaranteed to sit within `[margin, viewportWidth -
 * margin]`. If the overlay is wider than the viewport, we fall back to
 * pinning it to the left margin (degenerate but never broken).
 */
export function clampedAnchorLeft(
	selectionCenter: number,
	overlayWidth: number,
	viewportWidth: number,
	margin = 8
): number {
	const half = overlayWidth / 2;
	const minLeft = margin + half;
	const maxLeft = viewportWidth - margin - half;
	if (minLeft > maxLeft) {
		// Overlay wider than viewport — pin to the left margin so the user
		// at least sees the start of the controls. They can scroll if needed.
		return margin + half;
	}
	return Math.min(Math.max(selectionCenter, minLeft), maxLeft);
}
