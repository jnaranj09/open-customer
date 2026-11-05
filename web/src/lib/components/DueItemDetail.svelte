<script lang="ts">
	import { resolve } from '$app/paths';
	import type { DueItem } from '../api/schemas';
	import { t } from '../i18n';
	import { canSend, type Sendable } from '../pending';
	import { formatDayShort, todayIn } from '../time';
	import Button from './Button.svelte';
	import WriteButton from './WriteButton.svelte';
	import { buttonClasses } from './buttonStyle';

	// A due item with its actions: the client, the full message and Enviar / Omitir, or the booking
	// prompt (CUJ 5) with "Agregar a la cita", "Enviar mensaje" and "Omitir". A client without a
	// phone gets "Agregar teléfono" in place of the send button.
	let {
		item,
		zone,
		offline,
		busy,
		skipBlocked,
		onsend,
		onskip,
		onadd
	}: {
		item: DueItem;
		zone: string;
		offline: boolean;
		busy: boolean;
		/** Another "Omitir" is still inside its "Deshacer" window. */
		skipBlocked: boolean;
		onsend: (item: Sendable) => void;
		onskip: (item: DueItem) => void;
		onadd: (item: DueItem) => void;
	} = $props();

	const prompt = $derived(item.mode === 'booking_prompt' ? item.prompt : null);
	const promptText = $derived(
		prompt === null
			? null
			: t('due_prompt', {
					name: item.client_name,
					day: formatDayShort(todayIn(zone, new Date(prompt.starts_at))),
					services: prompt.services.join(', '),
					service: prompt.return_service_name
				})
	);

	// A button she must not use says why (ui.md): offline first, then a skip still undoable. Enviar
	// and the booking button are `WriteButton`s, which say the offline reason themselves.
	const skipOff = $derived(
		offline
			? { disabled: true as const, disabledReason: t('offline_cannot_save') }
			: skipBlocked
				? { disabled: true as const, disabledReason: t('due_skip_wait') }
				: {}
	);
</script>

<article class="flex flex-col gap-4">
	<h2 class="text-xl font-bold text-ink">{item.client_name}</h2>
	{#if promptText !== null}
		<p class="rounded-md bg-glass-tint px-4 py-3 text-base font-bold text-ink">{promptText}</p>
	{/if}
	<p class="text-base whitespace-pre-wrap text-ink">{item.text}</p>
	<div class="flex flex-wrap items-start gap-3">
		{#if prompt !== null}
			<div class="min-w-40 flex-1">
				<WriteButton
					variant="primary"
					{busy}
					onclick={() => {
						onadd(item);
					}}
				>
					{t('appointment_add_service')}
				</WriteButton>
			</div>
		{/if}
		<div class="min-w-40 flex-1">
			{#if canSend(item)}
				<WriteButton
					variant={prompt === null ? 'primary' : 'secondary'}
					{busy}
					onclick={() => {
						onsend(item);
					}}
				>
					{prompt === null ? t('due_send') : t('due_send_message')}
				</WriteButton>
			{:else}
				<a
					href={resolve('/clientas/[id]', { id: item.client_id })}
					class="{buttonClasses('secondary')} w-full"
				>
					{t('due_add_phone')}
				</a>
			{/if}
		</div>
		<div class="min-w-40 flex-1">
			<Button
				variant="ghost"
				{busy}
				{...skipOff}
				onclick={() => {
					onskip(item);
				}}
			>
				{t('due_skip')}
			</Button>
		</div>
	</div>
</article>
