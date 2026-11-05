<script lang="ts">
	import type { ResolvedPathname } from '$app/types';
	import Icon from '../icons/Icon.svelte';
	import { buttonClasses } from './buttonStyle';
	import WriteLink from '../WriteLink.svelte';

	// `href` is already resolved by the caller (`resolve`). `creates` marks an action that
	// opens a form to create something: offline it is off and says why.
	let {
		title,
		body,
		actionLabel,
		href,
		creates = false
	}: {
		title: string;
		body: string;
		actionLabel?: string;
		href?: ResolvedPathname;
		creates?: boolean;
	} = $props();
</script>

<div class="flex flex-col items-center gap-2 p-6 text-center">
	<Icon name="sparkles" class="size-8 text-accent-700" />
	<h2 class="text-xl font-bold text-ink">{title}</h2>
	<p class="text-sm text-ink-muted">{body}</p>
	{#if actionLabel !== undefined && href !== undefined}
		{#if creates}
			<div class="mt-2 w-full max-w-64">
				<WriteLink {href} variant="primary">{actionLabel}</WriteLink>
			</div>
		{:else}
			<a {href} class="{buttonClasses('primary')} mt-2">{actionLabel}</a>
		{/if}
	{/if}
</div>
