<script lang="ts">
	import type { Snippet } from 'svelte';
	import { resolve } from '$app/paths';
	import TabNav from '../../lib/components/ui/TabNav.svelte';
	import type { Tab } from '../../lib/components/ui/tabNav';
	import { t } from '../../lib/i18n';

	// Enviar (reminders-and-replies design, "Tabs and places"): the heading over two tabs, the
	// reminders to send today and her quick replies. /enviar is exact: it is a prefix of
	// /enviar/respuestas.
	let { children }: { children: Snippet } = $props();

	const TABS: readonly Tab[] = [
		{ href: resolve('/enviar'), label: 'tab_reminders', match: 'exact' },
		{ href: resolve('/enviar/respuestas'), label: 'tab_replies', match: 'exact' }
	];
</script>

<div class="flex flex-col gap-4">
	<h1 class="flex min-h-12 items-center text-3xl font-extrabold text-ink">{t('send_title')}</h1>
	<TabNav label={t('send_tabs_label')} tabs={TABS} />
	{@render children()}
</div>
