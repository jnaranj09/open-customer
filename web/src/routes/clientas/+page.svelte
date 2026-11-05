<script lang="ts">
	import Card from '../../lib/components/ui/Card.svelte';
	import EmptyState from '../../lib/components/ui/EmptyState.svelte';
	import { layout } from '../../lib/layout';
	import { t } from '../../lib/i18n';
</script>

<!-- On compact the list in the layout is this screen; on wide it sits beside this empty state. -->
{#if layout.current === 'wide'}
	<Card radius="xl" pad="section">
		<EmptyState title={t('clients_pick_title')} body={t('clients_pick_body')} />
	</Card>
{/if}
