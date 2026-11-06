<script lang="ts">
	import type { Rule, Service, Template } from '../api/schemas';
	import { t } from '../i18n';
	import { ruleSentence, ruleWarnings } from '../rules';
	import ListRow from './ui/ListRow.svelte';

	// One reminder rule as a sentence ("Enviar Relleno 21 días después de Acrílico, para agendar
	// Relleno"), with a warning under it while its template, service or return service is archived.
	let {
		rule,
		templates,
		services,
		onclick
	}: {
		rule: Rule;
		templates: readonly Template[];
		services: readonly Service[];
		onclick: () => void;
	} = $props();

	const warnings = $derived(ruleWarnings(rule, templates, services).map((key) => t(key)));
</script>

<ListRow
	title={ruleSentence(rule, templates, services)}
	{...warnings.length === 0 ? {} : { warning: warnings.join(' · ') }}
	{onclick}
/>
