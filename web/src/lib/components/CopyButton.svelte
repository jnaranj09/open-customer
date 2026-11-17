<script lang="ts">
	import { onDestroy } from 'svelte';
	import { copiedFlag, copyText } from '../copy';
	import { t } from '../i18n';
	import { toaster } from '../toaster.svelte';
	import Icon from './icons/Icon.svelte';
	import Button from './ui/Button.svelte';
	import type { ButtonVariant } from './ui/buttonStyle';

	// "Copiar" (mobile-polish design, section 3): writes `text` to the clipboard inside the tap
	// (lib/copy.ts). A copy says "Copiado" twice: the button shows the check icon and "Copiado" for
	// COPIED_MS, and the app's toast says it. A failure is the toast `copy_failed` (press and hold the
	// text). The toast's host is the app's live region (Toast.svelte), so it is also what a screen
	// reader hears, once.
	let { text, variant }: { text: string; variant: ButtonVariant } = $props();

	let copied = $state(false);
	const copiedNote = copiedFlag((on) => {
		copied = on;
	});

	function copy(): void {
		copyText(
			text,
			() => {
				copiedNote.show();
				toaster.say('copied');
			},
			() => {
				toaster.say('copy_failed');
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
		{t('copy')}
	{/if}
</Button>
