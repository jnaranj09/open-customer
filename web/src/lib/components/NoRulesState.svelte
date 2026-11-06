<script lang="ts">
	import Card from './ui/Card.svelte';
	import { resolve } from '$app/paths';
	import { t } from '../i18n';
	import EmptyState from './ui/EmptyState.svelte';
</script>

<!-- No reminder rules yet: leads to Ajustes → Recordatorios (shown by Hoy's card and by "Para enviar"). -->
<Card>
	<EmptyState
		title={t('due_no_rules_title')}
		body={t('due_no_rules_body')}
		actionLabel={t('due_no_rules_action')}
		href={resolve('/ajustes/recordatorios')}
	/>
</Card>
