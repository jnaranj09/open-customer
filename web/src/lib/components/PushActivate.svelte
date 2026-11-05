<script lang="ts">
	import { t } from '../i18n';
	import { pushControl } from '../pushControl.svelte';
	import Button from './ui/Button.svelte';
	import Icon from './icons/Icon.svelte';
</script>

<!-- The "Activar notificaciones" button, shown only while push needs her tap. -->
{#if pushControl.state === 'needs_activation'}
	<div class="flex flex-col gap-2">
		<Button variant="ghost" busy={pushControl.busy} onclick={() => void pushControl.activate()}>
			<Icon name="bell" />
			{t('push_activate')}
		</Button>
		{#if pushControl.error !== null}
			<p role="alert" class="text-sm text-error">{pushControl.error}</p>
		{/if}
	</div>
{/if}
