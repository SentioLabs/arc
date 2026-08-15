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

/**
 * Whether two measured `{commentId: top}` maps describe the same layout.
 *
 * Measurement is driven by a ResizeObserver, so it runs on every layout change
 * in the document column or the rail — and most of those move no highlight at
 * all. Storing an equal-but-freshly-allocated map would still invalidate the
 * rail's derived state and re-render every card, so the measuring code compares
 * first and only writes on a real change.
 */
export function railTopsEqual(a: Record<string, number>, b: Record<string, number>): boolean {
	const keys = Object.keys(a);
	if (keys.length !== Object.keys(b).length) return false;
	return keys.every((k) => Object.hasOwn(b, k) && a[k] === b[k]);
}

/**
 * Compute margin-card vertical positions (Google-Docs style).
 *
 * Baseline pass: each card wants its anchor's top; a forward sweep pushes
 * colliding cards down. When a card is active, it is pinned exactly at its
 * anchor: cards BEFORE it are swept backward (pushed up, clamped at 0, and
 * if clamping reintroduces overlap the excess ripples forward again), cards
 * AFTER it sweep forward from the pinned position.
 */
export function computeCardTops(
	rects: { top: number; height: number }[],
	activeIdx: number | null,
	gap = 12
): number[] {
	const n = rects.length;
	if (n === 0) return [];
	const tops = new Array<number>(n);

	const forwardFrom = (start: number, floor: number) => {
		let cursor = floor;
		for (let i = start; i < n; i++) {
			tops[i] = Math.max(rects[i].top, cursor);
			cursor = tops[i] + rects[i].height + gap;
		}
	};

	if (activeIdx === null || activeIdx < 0 || activeIdx >= n) {
		forwardFrom(0, 0);
		return tops;
	}

	// Pin the active card.
	tops[activeIdx] = Math.max(0, rects[activeIdx].top);

	// Backward sweep above the active card: each earlier card's bottom must
	// clear the following card's top.
	let ceiling = tops[activeIdx];
	for (let i = activeIdx - 1; i >= 0; i--) {
		const wanted = Math.min(rects[i].top, ceiling - gap - rects[i].height);
		tops[i] = Math.max(0, wanted);
		ceiling = tops[i];
	}
	// Clamping at 0 can reintroduce overlap among the upper cluster — resolve
	// by sweeping forward through the pre-active range, then re-pin active if
	// the cluster now reaches past it, pushing the tail down from there.
	let cursor = tops[0] + rects[0].height + gap;
	for (let i = 1; i < activeIdx; i++) {
		tops[i] = Math.max(tops[i], cursor);
		cursor = tops[i] + rects[i].height + gap;
	}
	if (activeIdx > 0) {
		tops[activeIdx] = Math.max(tops[activeIdx], cursor);
	}

	forwardFrom(activeIdx + 1, tops[activeIdx] + rects[activeIdx].height + gap);
	return tops;
}
