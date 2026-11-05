<script lang="ts">
	import type { ResolvedPathname } from '$app/types';
	import type { Snippet } from 'svelte';
	import Icon from './icons/Icon.svelte';

	// `href` is already resolved by the caller (`resolve`), so a dynamic route like
	// `/clientas/[id]` can pass its values. A row with `onclick` instead is a button, for a list
	// whose rows choose what a pane beside it shows.
	let {
		href,
		onclick,
		title,
		subtitle,
		trailing,
		selected = false
	}: {
		href?: ResolvedPathname;
		onclick?: () => void;
		title: string;
		subtitle?: string;
		trailing?: Snippet;
		selected?: boolean;
	} = $props();
</script>

<svelte:element
	this={href !== undefined ? 'a' : onclick !== undefined ? 'button' : 'div'}
	{href}
	type={href === undefined && onclick !== undefined ? 'button' : undefined}
	{onclick}
	role={href === undefined && onclick !== undefined ? 'button' : undefined}
	aria-current={selected ? 'true' : undefined}
	class="flex min-h-tap w-full items-center gap-3 rounded-md px-3 py-3.5 text-left {selected
		? 'bg-glass-tint'
		: ''} {href === undefined && onclick === undefined ? '' : 'hover:bg-glass'}"
>
	<span class="flex min-w-0 flex-1 flex-col gap-0.5">
		<span class="text-base font-bold text-ink">{title}</span>
		{#if subtitle !== undefined}
			<span class="text-sm text-ink-muted">{subtitle}</span>
		{/if}
	</span>
	{#if trailing}
		{@render trailing()}
	{:else if href !== undefined || onclick !== undefined}
		<Icon name="chevron-right" class="size-4.5 text-ink-muted" />
	{/if}
</svelte:element>
