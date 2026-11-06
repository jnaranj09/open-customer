<script lang="ts" module>
	/** The .pen TextButton variants: `accent` (a secondary action in the accent ink), `danger`
	 *  (the same in the error ink), `back` (a chevron and a label, flush with the left edge). */
	export type TextButtonVariant = 'accent' | 'danger' | 'back';
</script>

<script lang="ts">
	import type { ResolvedPathname } from '$app/types';
	import type { Snippet } from 'svelte';
	import Icon from '../icons/Icon.svelte';

	// A text-only pill button that sizes to its label (a `Button` fills its parent). With `href` it
	// is a link. `class` is for layout only (`self-start`, `mt-1`).
	let {
		variant,
		href,
		onclick,
		disabled = false,
		class: layoutClass = '',
		children
	}: {
		variant: TextButtonVariant;
		href?: ResolvedPathname;
		onclick?: () => void;
		disabled?: boolean;
		class?: string;
		children: Snippet;
	} = $props();

	const LOOK: Record<TextButtonVariant, string> = {
		accent: 'px-4 text-accent-text',
		danger: 'px-4 text-error',
		back: 'inline-flex items-center gap-1 pr-4 text-accent-text'
	};
	const classes = $derived(
		`min-h-tap rounded-pill text-sm font-bold hover:bg-accent-100 disabled:opacity-40 ${LOOK[variant]} ${layoutClass}`
	);
</script>

{#snippet label()}
	{#if variant === 'back'}
		<Icon name="chevron-left" class="size-5" />
	{/if}
	{@render children()}
{/snippet}

{#if href !== undefined}
	<a {href} class={classes}>{@render label()}</a>
{:else}
	<button type="button" {onclick} {disabled} class={classes}>{@render label()}</button>
{/if}
