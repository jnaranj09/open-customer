<script lang="ts">
	import type { Snippet } from 'svelte';
	import { t } from '../../lib/i18n';
	import { provideHeaderAction } from './header.svelte';

	// Servicios (quick-quotes design, section 5), a top-level place: the heading and, at its right
	// on wide, the action its page registered through `useHeaderAction`. On the phone the slot is
	// empty (no row is drawn): the page uses the floating "+". The tab links arrive with the Cotizar tab; until then the catalog
	// is the only page.
	let { children }: { children: Snippet } = $props();

	const action = provideHeaderAction();
</script>

<div class="flex flex-col gap-4">
	<header class="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
		<h1 class="flex min-h-12 items-center text-3xl font-extrabold text-ink">
			{t('services_title')}
		</h1>
		{#if action.snippet !== null}
			<div class="w-44 shrink-0">{@render action.snippet()}</div>
		{/if}
	</header>
	{@render children()}
</div>
