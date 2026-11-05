<script lang="ts">
	import Callout from './ui/Callout.svelte';
	import TextButton from './ui/TextButton.svelte';
	import Card from './ui/Card.svelte';
	import type { Snippet } from 'svelte';
	import { t } from '../i18n';
	import Button from './ui/Button.svelte';
	import WriteButton from './WriteButton.svelte';

	// The card both record forms sit in (D2: one glass card, up to 760 px on wide): the notice of a
	// recovered draft with "Empezar de nuevo", the fields, and the save button with "Cancelar"
	// beside it on wide. Saving needs the server, so offline the button says why it is off.
	let {
		restored = false,
		label,
		busy,
		hideSave = false,
		onsubmit,
		onstartover,
		oncancel,
		children
	}: {
		/** A draft was recovered: shows the notice with "Empezar de nuevo" (`onstartover`). */
		restored?: boolean;
		label: string;
		busy: boolean;
		/** The save button is off the card while another button answers a warning. */
		hideSave?: boolean;
		onsubmit: () => void;
		onstartover?: () => void;
		oncancel?: (() => void) | undefined;
		children: Snippet;
	} = $props();
</script>

<Card
	as="form"
	radius="xl"
	pad="section"
	class="flex flex-col gap-4 lg:max-w-190"
	onsubmit={(e: SubmitEvent) => {
		e.preventDefault();
		onsubmit();
	}}
>
	{#if restored && onstartover !== undefined}
		<Callout pad="trail" class="flex items-center gap-3 text-sm">
			<p class="flex-1">{t('draft_restored')}</p>
			<TextButton variant="accent" onclick={onstartover}>{t('draft_discard')}</TextButton>
		</Callout>
	{/if}

	{@render children()}

	<div class="flex flex-col gap-3 lg:flex-row-reverse lg:justify-start">
		{#if !hideSave}
			<div class="lg:w-56">
				<WriteButton variant="strong" type="submit" {busy}>{label}</WriteButton>
			</div>
		{/if}
		{#if oncancel !== undefined}
			<div class="lg:w-40">
				<Button variant="glass" onclick={oncancel}>{t('cancel')}</Button>
			</div>
		{/if}
	</div>
</Card>
