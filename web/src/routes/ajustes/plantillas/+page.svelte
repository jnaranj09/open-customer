<script lang="ts">
	import EmptyState from '../../../lib/components/EmptyState.svelte';
	import { t } from '../../../lib/i18n';
</script>

<!-- Beside the list on a wide page; the layout hides this pane on a narrow one. -->
<div class="glass-card rounded-lg">
	<EmptyState title={t('templates_pick_title')} body={t('templates_pick_body')} />
</div>
