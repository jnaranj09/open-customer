<script lang="ts">
	import { resolve } from '$app/paths';
	import type { RouteIdWithSearchOrHash } from '$app/types';
	import Icon from './icons/Icon.svelte';
	import { buttonClasses } from './buttonStyle';

	let {
		title,
		body,
		actionLabel,
		href
	}: { title: string; body: string; actionLabel?: string; href?: RouteIdWithSearchOrHash } =
		$props();
</script>

<div class="flex flex-col items-center gap-2 p-6 text-center">
	<Icon name="sparkles" class="size-8 text-accent-700" />
	<h2 class="text-xl font-bold text-ink">{title}</h2>
	<p class="text-sm text-ink-muted">{body}</p>
	{#if actionLabel !== undefined && href !== undefined}
		<a href={resolve(href)} class="{buttonClasses('primary')} mt-2">{actionLabel}</a>
	{/if}
</div>
