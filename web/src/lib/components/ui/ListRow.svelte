<script lang="ts">
	import type { ResolvedPathname } from '$app/types';
	import type { Snippet } from 'svelte';
	import Icon from '../icons/Icon.svelte';

	// `href` is already resolved by the caller (`resolve`), so a dynamic route like
	// `/clientas/[id]` can pass its values. With `onclick` instead of `href` the row is a button.
	let {
		href,
		onclick,
		title,
		subtitle,
		warning,
		trailing,
		selected = false
	}: {
		href?: ResolvedPathname;
		onclick?: () => void;
		title: string;
		subtitle?: string;
		/** A problem with the row, in the error color under the subtitle. */
		warning?: string;
		trailing?: Snippet;
		selected?: boolean;
	} = $props();

	const classes = $derived(
		`flex min-h-tap w-full items-center gap-3 rounded-md px-3 py-3.5 text-left ${
			selected ? 'bg-glass-tint' : ''
		} ${href === undefined && onclick === undefined ? '' : 'hover:bg-glass'}`
	);
</script>

{#snippet inner()}
	<span class="flex min-w-0 flex-1 flex-col gap-0.5">
		<span class="text-base font-bold text-ink">{title}</span>
		{#if subtitle !== undefined}
			<span class="text-sm text-ink-muted">{subtitle}</span>
		{/if}
		{#if warning !== undefined}
			<span class="text-sm font-bold text-error">{warning}</span>
		{/if}
	</span>
	{#if trailing}
		{@render trailing()}
	{:else if href !== undefined || onclick !== undefined}
		<Icon name="chevron-right" class="size-4.5 text-ink-muted" />
	{/if}
{/snippet}

{#if href !== undefined}
	<a {href} aria-current={selected ? 'true' : undefined} class={classes}>{@render inner()}</a>
{:else if onclick !== undefined}
	<button type="button" {onclick} class={classes}>{@render inner()}</button>
{:else}
	<div class={classes}>{@render inner()}</div>
{/if}
