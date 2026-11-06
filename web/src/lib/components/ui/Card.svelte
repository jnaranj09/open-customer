<script lang="ts" module>
	/** The .pen GlassCard variants: `default` (glass), `strong` (denser glass for dense text and
	 *  panels), `tint` (the selected / attention tint). */
	export type CardVariant = 'default' | 'strong' | 'tint';
	/** Inner spacing. `list` holds rows that bring their own padding; `note` is a short line of
	 *  small text; `message` is one line of status or error text; `trail` is text with a trailing
	 *  action button (`field` the same at the height of a form field); `section` grows on wide. */
	export type CardPad =
		'none' | 'list' | 'note' | 'field' | 'trail' | 'message' | 'md' | 'lg' | 'section';
	export type CardRadius = 'none' | 'sm' | 'lg' | 'xl' | 'pill';
	export type CardElement =
		'div' | 'section' | 'form' | 'ul' | 'li' | 'p' | 'a' | 'button' | 'aside';
</script>

<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { HTMLAnchorAttributes, HTMLAttributes } from 'svelte/elements';

	// The one glass surface (.claude/rules/ui.md "UI pipeline"). Callers pick a variant, a padding
	// and a radius from the sets above; `class` is for layout only (flex, gap, width, position,
	// text), never padding, radius or surface: lint rejects those outside this folder.
	type Props = Omit<HTMLAttributes<HTMLElement>, 'class' | 'children'> & {
		variant?: CardVariant;
		pad?: CardPad;
		radius?: CardRadius;
		as?: CardElement;
		class?: string;
		href?: HTMLAnchorAttributes['href'];
		type?: 'button' | 'submit';
		/** The picked one of a list: an accent outline (a state, not a look). */
		selected?: boolean;
		/** The rendered element, for a caller that scrolls or measures it. */
		ref?: HTMLElement | undefined;
		children: Snippet;
	};

	let {
		variant = 'default',
		pad = 'none',
		radius = 'lg',
		as = 'div',
		class: layoutClass = '',
		selected = false,
		ref = $bindable(),
		children,
		...rest
	}: Props = $props();

	const SURFACE: Record<CardVariant, string> = {
		default: 'glass-card',
		strong: 'glass-panel',
		tint: 'glass-card bg-glass-tint'
	};
	const PAD: Record<CardPad, string> = {
		none: '',
		list: 'p-2',
		note: 'px-4 py-2',
		field: 'py-1 pr-1 pl-4',
		trail: 'py-2 pr-2 pl-5',
		message: 'px-5 py-3',
		md: 'p-5',
		lg: 'p-6',
		section: 'p-5 lg:p-8'
	};
	const RADIUS: Record<CardRadius, string> = {
		none: 'rounded-none',
		sm: 'rounded-sm',
		lg: 'rounded-lg',
		xl: 'rounded-xl',
		pill: 'rounded-pill'
	};
</script>

<svelte:element
	this={as}
	bind:this={ref}
	{...rest}
	class="{SURFACE[variant]} {RADIUS[radius]} {PAD[pad]} {selected
		? 'outline-2 outline-accent'
		: ''} {layoutClass}"
>
	{@render children()}
</svelte:element>
