// The action slot of the Servicios header (quick-quotes design, section 5): the layout owns the
// heading row and renders what a tab page registered, so the page never repeats the heading.
import { getContext, onDestroy, setContext, type Snippet } from 'svelte';

const KEY = Symbol('servicios-header-action');

/** What the layout renders at the right of the heading; `null` is no action. */
export type HeaderAction = { snippet: Snippet | null };

/** Called by the layout: makes the slot available to the pages below and returns it. */
export function provideHeaderAction(): HeaderAction {
	const holder = $state<HeaderAction>({ snippet: null });
	return setContext(KEY, holder);
}

/**
 * Called by a page: shows `snippet` in the slot while `show()` is true and until the page is
 * destroyed; while it is false the slot stays empty, so the layout draws no row for it.
 */
export function useHeaderAction(snippet: Snippet, show: () => boolean): void {
	const holder = getContext<HeaderAction | undefined>(KEY);
	if (holder === undefined) {
		throw new Error('useHeaderAction needs the Servicios layout');
	}
	$effect.pre(() => {
		if (show()) {
			holder.snippet = snippet;
		} else if (holder.snippet === snippet) {
			holder.snippet = null;
		}
	});
	onDestroy(() => {
		// A page that replaced this one has already registered its own: leave that alone.
		if (holder.snippet === snippet) {
			holder.snippet = null;
		}
	});
}
