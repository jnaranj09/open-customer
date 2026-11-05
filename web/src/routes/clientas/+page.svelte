<script lang="ts">
	import EmptyState from '../../lib/components/EmptyState.svelte';
	import { layout } from '../../lib/layout';
	import { t } from '../../lib/i18n';
</script>

<!-- On compact the list in the layout is this screen; on wide it sits beside this empty state. -->
{#if layout.current === 'wide'}
	<div class="glass-card rounded-xl py-12">
		<EmptyState title={t('clients_pick_title')} body={t('clients_pick_body')} />
	</div>
{/if}
