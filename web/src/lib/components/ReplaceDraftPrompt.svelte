<script lang="ts">
	import { t } from '../i18n';
	import Button from './ui/Button.svelte';
	import Callout from './ui/Callout.svelte';

	// The question a hand-off into Nueva cita asks when an unsaved booking is already there
	// (`handOff` answered `needs_confirm`): replace it, or keep it. Cotizar and "Agendar próxima
	// cita" show it.
	let {
		busy,
		onreplace,
		oncancel
	}: { busy: boolean; onreplace: () => void; oncancel: () => void } = $props();
</script>

<Callout role="status" class="flex flex-col gap-3 text-sm">
	<p>{t('draft_replace_prompt')}</p>
	<div class="grid grid-cols-2 gap-3">
		<Button variant="strong" {busy} onclick={onreplace}>{t('draft_replace')}</Button>
		<Button variant="glass" {busy} onclick={oncancel}>{t('cancel')}</Button>
	</div>
</Callout>
