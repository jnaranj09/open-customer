<script lang="ts">
	import { t } from '../i18n';
	import Button from './ui/Button.svelte';
	import WriteButton from './WriteButton.svelte';

	// The "¿Incluir atrasados?" answers, inside a sheet titled `overdue_title`: a rule change that
	// can add items, and the handover of an archive or restore (reminders-and-replies design,
	// "Handover"). `onanswer(false)` is "Solo de hoy en adelante", `onanswer(true)` "Incluir
	// atrasados"; "Cancelar" goes back.
	let {
		busy,
		onanswer,
		oncancel
	}: { busy: boolean; onanswer: (include: boolean) => void; oncancel: () => void } = $props();

	// Moves focus to the first answer when the question appears.
	function focusFirst(node: HTMLElement): void {
		node.querySelector<HTMLElement>('button')?.focus();
	}
</script>

<div class="flex flex-col gap-3" {@attach focusFirst}>
	<p class="text-base text-ink-muted">{t('overdue_body')}</p>
	<WriteButton
		variant="strong"
		{busy}
		onclick={() => {
			onanswer(false);
		}}>{t('overdue_from_today')}</WriteButton
	>
	<WriteButton
		variant="glass"
		{busy}
		onclick={() => {
			onanswer(true);
		}}>{t('overdue_include')}</WriteButton
	>
	<Button variant="ghost" onclick={oncancel}>{t('cancel')}</Button>
</div>
