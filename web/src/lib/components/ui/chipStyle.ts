/** A variable chip inside the template editor's text (`chipsDom.ts` builds the element by hand,
 *  outside Svelte, so it takes the classes as a string). The look of the .pen Chip/Selected, small
 *  enough to sit in a line of text, with a 44 px tap target: a transparent 10 px border above and
 *  below (the fill stops at the padding, `bg-clip-padding`, and the edge is an inset ring) makes
 *  the box 44 px tall, and a matching negative margin keeps the line of text its usual height. */
export const TOKEN_CHIP_CLASSES =
	'mx-0.5 -my-2.5 inline-block cursor-pointer select-none rounded-pill border-x-0 border-y-10 border-transparent bg-accent-100 bg-clip-padding px-3 py-0.5 align-baseline text-sm font-bold text-accent-text ring-1 ring-accent-300 ring-inset hover:bg-accent-300';
