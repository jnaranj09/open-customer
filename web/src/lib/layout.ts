import { MediaQuery } from 'svelte/reactivity';

/** `compact` is the phone layout; `wide` is the desktop layout (design doc "Screen layouts"). */
export type Layout = 'compact' | 'wide';

/** The window width where `wide` starts. */
export const WIDE_MIN_PX = 1024;

// Created on first read, not at import: `matchMedia` needs a window, and the module is imported
// where there is none (tests, the prerender pass).
let wide: MediaQuery | undefined;

/** The layout now, following the window size. Structure that differs between layouts reads this;
 *  purely visual differences use Tailwind `lg:` (1024 px, the same boundary). */
export const layout: { readonly current: Layout } = {
	get current(): Layout {
		if (typeof window === 'undefined') return 'compact';
		wide ??= new MediaQuery(`min-width: ${String(WIDE_MIN_PX)}px`);
		return wide.current ? 'wide' : 'compact';
	}
};
