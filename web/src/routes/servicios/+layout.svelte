<script lang="ts">
	import type { Snippet } from 'svelte';
	import { resolve } from '$app/paths';
	import TabNav from '../../lib/components/ui/TabNav.svelte';
	import type { Tab } from '../../lib/components/ui/tabNav';
	import { t } from '../../lib/i18n';
	import { provideHeaderAction } from './header.svelte';

	// Servicios (quick-quotes design, section 5), a top-level place: the heading and, at its right
	// on wide, the action its page registered through `useHeaderAction`. On the phone the slot is
	// empty (no row is drawn): the page uses the floating "+". Under it, the two tabs (`TabNav`).
	let { children }: { children: Snippet } = $props();

	const action = provideHeaderAction();
	// /servicios is exact: it is a prefix of /servicios/catalogo.
	const TABS: readonly Tab[] = [
		{ href: resolve('/servicios'), label: 'services_tab_quote', match: 'exact' },
		{ href: resolve('/servicios/catalogo'), label: 'services_tab_catalog', match: 'exact' }
	];
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
	<TabNav label={t('services_tabs_label')} tabs={TABS} />
	{@render children()}
</div>
