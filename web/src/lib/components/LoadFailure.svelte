<script lang="ts">
	import { errorText, type ApiError } from '../api/errors';
	import { t } from '../i18n';
	import Button from './Button.svelte';

	// A record that could not be read: why, and "Reintentar".
	let { error, onretry }: { error: ApiError; onretry: () => void } = $props();
</script>

<div role="alert" class="glass-card flex flex-col gap-3 rounded-lg px-5 py-3">
	<p class="text-base text-error">{errorText(error)}</p>
	<Button variant="ghost" onclick={onretry}>{t('agenda_retry')}</Button>
</div>
