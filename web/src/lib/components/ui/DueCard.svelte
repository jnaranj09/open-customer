<script lang="ts" module>
	/** The .pen DueCard variants, picked by the item's mode: the message to send, or the booking
	 *  prompt (CUJ 5). */
	export type DueCardVariant = 'message' | 'bookingPrompt';
</script>

<script lang="ts">
	import Callout from './Callout.svelte';
	import { resolve } from '$app/paths';
	import type { DueItem } from '../../api/schemas';
	import { t } from '../../i18n';
	import { canSend, type Sendable } from '../../pending';
	import { formatDayShort, todayIn } from '../../time';
	import Button from './Button.svelte';
	import { OFFLINE_REASON_ID } from './offlineReason';
	import WriteButton from '../WriteButton.svelte';
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
		skipReasonId,
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
		/** The id of the screen's one line that says why "Omitir" waits (`due_skip_wait`). */
		skipReasonId: string;
		onsend: (item: Sendable) => void;
		onskip: (item: DueItem) => void;
		onadd: (item: DueItem) => void;
	} = $props();

	const variant: DueCardVariant = $derived(
		item.mode === 'booking_prompt' ? 'bookingPrompt' : 'message'
	);
	const prompt = $derived(variant === 'bookingPrompt' ? item.prompt : null);
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

	// A button she must not use says why (ui.md): offline first, then a skip still undoable. The
	// reason is said once per screen, not under every card of the list: offline the banner's line,
	// while a skip waits the screen's own line (`skipReasonId`). Enviar and the booking button are
	// `WriteButton`s pointing at the banner the same way.
	const skipOff = $derived(
		offline
			? { disabled: true as const, reasonId: OFFLINE_REASON_ID }
			: skipBlocked
				? { disabled: true as const, reasonId: skipReasonId }
				: {}
	);
</script>

<article class="flex flex-col gap-4">
	<h2 class="text-xl font-bold text-ink">{item.client_name}</h2>
	{#if promptText !== null}
		<Callout as="p" class="text-base font-bold">{promptText}</Callout>
	{/if}
	<p class="text-base whitespace-pre-wrap text-ink">{item.text}</p>
	<div class="flex flex-wrap items-start gap-3">
		{#if prompt !== null}
			<div class="min-w-40 flex-1">
				<WriteButton
					variant="strong"
					sharedOfflineReason
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
					variant={prompt === null ? 'strong' : 'glass'}
					sharedOfflineReason
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
					class="{buttonClasses('glass')} w-full"
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
