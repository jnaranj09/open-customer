<script lang="ts">
	import type { ResolvedPathname } from '$app/types';
	import type { Snippet } from 'svelte';
	import { cache } from '../cache/store.svelte';
	import { t } from '../i18n';
	import Button from './Button.svelte';
	import { buttonClasses, type ButtonVariant } from './buttonStyle';

	// A link to a screen that creates something ("Nueva cita", "Nueva clienta"). Offline the screen
	// could not save, so the link is a button that is off and says why.
	let {
		href,
		variant,
		width = 'w-full',
		children
	}: {
		href: ResolvedPathname;
		variant: ButtonVariant;
		width?: string;
		children: Snippet;
	} = $props();
</script>

<div class="shrink-0 {width}">
	{#if cache.offline}
		<Button {variant} disabled disabledReason={t('offline_cannot_save')}>
			{@render children()}
		</Button>
	{:else}
		<a {href} class="{buttonClasses(variant)} w-full">{@render children()}</a>
	{/if}
</div>
