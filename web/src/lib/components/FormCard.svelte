<script lang="ts">
	import type { Snippet } from 'svelte';
	import { cache } from '../cache/store.svelte';
	import { t } from '../i18n';
	import Button from './Button.svelte';

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

<form
	class="glass-card flex flex-col gap-4 rounded-xl p-5 lg:max-w-190 lg:p-8"
	onsubmit={(e) => {
		e.preventDefault();
		onsubmit();
	}}
>
	{#if restored && onstartover !== undefined}
		<div class="flex items-center gap-3 rounded-md bg-glass-tint py-1 pr-1 pl-4 text-sm text-ink">
			<p class="flex-1">{t('draft_restored')}</p>
			<button
				type="button"
				class="min-h-tap rounded-pill px-4 font-bold text-accent-700 hover:bg-accent-100"
				onclick={onstartover}
			>
				{t('draft_discard')}
			</button>
		</div>
	{/if}

	{@render children()}

	<div class="flex flex-col gap-3 lg:flex-row-reverse lg:justify-start">
		{#if !hideSave}
			<div class="lg:w-56">
				<Button
					variant="primary"
					type="submit"
					{busy}
					{...cache.offline
						? { disabled: true as const, disabledReason: t('offline_cannot_save') }
						: {}}
				>
					{label}
				</Button>
			</div>
		{/if}
		{#if oncancel !== undefined}
			<div class="lg:w-40">
				<Button variant="secondary" onclick={oncancel}>{t('cancel')}</Button>
			</div>
		{/if}
	</div>
</form>
