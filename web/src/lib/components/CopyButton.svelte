<script lang="ts">
	import { onDestroy } from 'svelte';
	import { copiedFlag, copyText } from '../copy';
	import { t } from '../i18n';
	import Icon from './icons/Icon.svelte';
	import Button from './ui/Button.svelte';
	import type { ButtonVariant } from './ui/buttonStyle';

	// "Copiar": writes `text` to the clipboard inside the tap (lib/copy.ts), then shows the check icon
	// and "Copiado" for COPIED_MS. `onresult` tells the caller whether the copy worked.
	let {
		text,
		variant,
		label = t('copy'),
		onresult
	}: {
		text: string;
		variant: ButtonVariant;
		/** The button's text before a copy. */
		label?: string;
		onresult: (ok: boolean) => void;
	} = $props();

	let copied = $state(false);
	const copiedNote = copiedFlag((on) => {
		copied = on;
	});

	function copy(): void {
		copyText(
			text,
			() => {
				onresult(true);
				copiedNote.show();
			},
			() => {
				onresult(false);
			}
		);
	}

	onDestroy(() => {
		copiedNote.stop();
	});
</script>

<Button {variant} onclick={copy}>
	{#if copied}
		<Icon name="check" class="size-4.5" />
		{t('copied')}
	{:else}
		{label}
	{/if}
</Button>
<!-- A message for screen readers only: the copy needs no toast. -->
<span class="sr-only" aria-live="polite">{copied ? t('copied') : ''}</span>
