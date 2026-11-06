// The page behind an open sheet is inert (.claude/rules/ui.md "Accessibility and touch basics"):
// no tap, no Tab, no screen reader reaches it. A sheet lives in its own child of <body>, so the
// layers are <body>'s children: every one but the top sheet's is made inert. A layer marked with
// LIVE_LAYER (the toast, whose "Deshacer" must stay tappable over a sheet) is left alone.

/** The attribute that keeps a layer of <body> out of the inert page. */
export const LIVE_LAYER = 'data-live-layer';

/** What this needs of an element, so a test can pass plain objects. */
export type Layer = { inert: boolean; hasAttribute: (name: string) => boolean };

/** Makes every layer but `keep` (the top sheet's) inert; null, no sheet open, frees them all.
 *  `held` remembers the layers this made inert, so it frees only those: one that was inert for
 *  another reason stays inert. */
export function inertBehind(layers: Iterable<Layer>, keep: Layer | null, held: Set<Layer>): void {
	const seen = new Set<Layer>();
	for (const layer of layers) {
		seen.add(layer);
		const live = keep === null || layer === keep || layer.hasAttribute(LIVE_LAYER);
		if (live && held.has(layer)) {
			layer.inert = false;
			held.delete(layer);
		} else if (!live && !layer.inert) {
			layer.inert = true;
			held.add(layer);
		}
	}
	// A sheet that closed has left <body>: forget it.
	for (const layer of held) {
		if (!seen.has(layer)) {
			held.delete(layer);
		}
	}
}

/** A Svelte attachment that moves an element to its own layer of <body>. A sheet or a toast is a
 *  fixed layer above the whole app, and a card with `backdrop-filter` (a glass card) is its own
 *  stacking context: declared inside one, it would sit under the tab bar, and it could not stay
 *  out of the inert page. */
export function toBody(node: HTMLElement): () => void {
	document.body.appendChild(node);
	return () => {
		node.remove();
	};
}
