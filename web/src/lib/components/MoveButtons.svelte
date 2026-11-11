<script lang="ts">
	import { t } from '../i18n';
	import TextButton from './ui/TextButton.svelte';

	// "Subir" and "Bajar" for one row of an ordered list (the attribute options, the quick replies).
	// The first row cannot go up and the last cannot go down; `onmove` gets the new index.
	let {
		index,
		count,
		disabled = false,
		onmove
	}: { index: number; count: number; disabled?: boolean; onmove: (to: number) => void } = $props();
</script>

<TextButton
	variant="accent"
	disabled={disabled || index === 0}
	onclick={() => {
		onmove(index - 1);
	}}>{t('option_up')}</TextButton
>
<TextButton
	variant="accent"
	disabled={disabled || index === count - 1}
	onclick={() => {
		onmove(index + 1);
	}}>{t('option_down')}</TextButton
>
