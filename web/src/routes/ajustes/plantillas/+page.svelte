<script lang="ts">
	import Card from '../../../lib/components/ui/Card.svelte';
	import EmptyState from '../../../lib/components/ui/EmptyState.svelte';
	import { t } from '../../../lib/i18n';
</script>

<!-- Beside the list on a wide page; the layout hides this pane on a narrow one. -->
<Card>
	<EmptyState title={t('templates_pick_title')} body={t('templates_pick_body')} />
</Card>
