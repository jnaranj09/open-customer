<script lang="ts">
	import type { Snippet } from 'svelte';
	import { resolve } from '$app/paths';
	import TabNav from '../../../lib/components/ui/TabNav.svelte';
	import type { Tab } from '../../../lib/components/ui/tabNav';
	import { t } from '../../../lib/i18n';

	// Ajustes → Recordatorios (reminders-and-replies design, "Tabs and places"): one heading over the
	// rules and the templates, as two tabs. The URLs stay /ajustes/recordatorios and
	// /ajustes/plantillas/…; Plantillas keeps its tab on a template and on the new-template form.
	let { children }: { children: Snippet } = $props();

	const TABS: readonly Tab[] = [
		{ href: resolve('/ajustes/(reglas)/recordatorios'), label: 'tab_rules', match: 'exact' },
		{ href: resolve('/ajustes/(reglas)/plantillas'), label: 'tab_templates', match: 'prefix' }
	];
</script>

<div class="flex flex-col gap-4">
	<h1 class="flex min-h-12 items-center text-3xl font-extrabold text-ink">
		{t('settings_rules')}
	</h1>
	<TabNav label={t('rules_tabs_label')} tabs={TABS} />
	{@render children()}
</div>
