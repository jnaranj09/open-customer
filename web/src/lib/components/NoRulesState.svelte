<script lang="ts">
	import { resolve } from '$app/paths';
	import { t } from '../i18n';
	import EmptyState from './EmptyState.svelte';
</script>

<!-- No reminder rules yet: leads to Ajustes (shown by Hoy's card and by "Para enviar"). -->
<div class="glass-card rounded-lg">
	<EmptyState
		title={t('due_no_rules_title')}
		body={t('due_no_rules_body')}
		actionLabel={t('due_no_rules_action')}
		href={resolve('/ajustes')}
	/>
</div>
