<script lang="ts">
	import Card from './ui/Card.svelte';
	import { errorText, type ApiError } from '../api/errors';
	import { t } from '../i18n';
	import Button from './ui/Button.svelte';

	// A record that could not be read: why, and "Reintentar".
	let { error, onretry }: { error: ApiError; onretry: () => void } = $props();
</script>

<Card role="alert" pad="message" class="flex flex-col gap-3">
	<p class="text-base text-error">{errorText(error)}</p>
	<Button variant="ghost" onclick={onretry}>{t('agenda_retry')}</Button>
</Card>
