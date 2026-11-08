<script lang="ts">
	import { onDestroy } from 'svelte';
	import { resolve } from '$app/paths';
	import type { Service } from '../api/schemas';
	import { activeServiceIds, removeService, tapService, type Line } from '../booking';
	import { draftKey } from '../cache/drafts';
	import { formDraft } from '../cache/formDraft.svelte';
	import { t } from '../i18n';
	import { layout } from '../layout';
	import { formatMoney } from '../money';
	import { QuoteDraftSchema, quoteText } from '../quote';
	import Icon from './icons/Icon.svelte';
	import ServicePicker from './ServicePicker.svelte';
	import Button from './ui/Button.svelte';
	import Callout from './ui/Callout.svelte';
	import Card from './ui/Card.svelte';
	import EmptyState from './ui/EmptyState.svelte';
	import IconButton from './ui/IconButton.svelte';
	import TextButton from './ui/TextButton.svelte';

	// The Cotizar tab (quick-quotes design, items 22 and 2-3): she picks the services somebody asked
	// about, reads the quote and taps Copiar to paste it into WhatsApp. The open quote is a draft, so
	// it is still there when she comes back. The text on screen is the text Copiar copies.
	let {
		services,
		settings
	}: {
		/** The active services only. */
		services: Service[];
		settings: { name: string; currency: string; locale: string };
	} = $props();

	/** How long the Copiar button says "Copiado". */
	const COPIED_MS = 2000;

	const activeIds = $derived(activeServiceIds(services));
	const wide = $derived(layout.current === 'wide');

	let lines = $state<Line[]>([]);
	// The message above the text: none, or the phone refused the copy.
	let message = $state<'none' | 'copy_failed'>('none');
	let copied = $state(false);
	let copiedTimer: ReturnType<typeof setTimeout> | undefined;

	const draft = formDraft(
		draftKey('cotizacion', 'new'),
		QuoteDraftSchema,
		{ lines: [] },
		() => ({ lines: $state.snapshot(lines) }),
		(saved) => {
			lines = saved.lines.filter((l) => activeIds.has(l.service_id));
		}
	);

	// Only the lines she can see: a service archived while the quote is open drops out of the text
	// (design doc D4).
	const visible = $derived(lines.filter((l) => activeIds.has(l.service_id)));
	const text = $derived(quoteText(visible, services, settings));
	const countOf = (id: string): number => lines.find((l) => l.service_id === id)?.count ?? 0;

	// Every pick goes through here, so the message goes with her next pick.
	function setLines(next: Line[]): void {
		message = 'none';
		lines = next;
	}

	function copyFailed(): void {
		message = 'copy_failed';
		console.warn('code=clipboard_failed');
	}

	function copyDone(): void {
		copied = true;
		clearTimeout(copiedTimer);
		copiedTimer = setTimeout(() => {
			copied = false;
		}, COPIED_MS);
	}

	// The write starts inside the tap, with nothing awaited before it: iOS allows it only then.
	function copy(): void {
		message = 'none';
		// Missing outside a secure context, which the DOM types do not show.
		const clipboard = navigator.clipboard as Clipboard | undefined;
		if (clipboard === undefined) {
			copyFailed();
			return;
		}
		try {
			void clipboard.writeText(text).then(copyDone, copyFailed);
		} catch {
			copyFailed();
		}
	}

	function startOver(): void {
		message = 'none';
		void draft.startOver();
	}

	onDestroy(() => {
		clearTimeout(copiedTimer);
	});
</script>

{#snippet notice()}
	{#if message === 'copy_failed'}
		<Callout role="alert" class="text-sm">{t('quote_copy_failed')}</Callout>
	{/if}
{/snippet}

{#snippet copyButton()}
	<Button variant="strong" onclick={copy}>
		{#if copied}
			<Icon name="check" class="size-4.5" />
			{t('quote_copied')}
		{:else}
			{t('quote_copy')}
		{/if}
	</Button>
{/snippet}

{#snippet startOverButton()}
	<TextButton variant="accent" onclick={startOver} class="self-center">{t('quote_new')}</TextButton>
{/snippet}

<!-- The wide catalog table: a stepper per row, "+" alone for a service not picked yet. -->
{#snippet table()}
	<Card pad="list">
		<table class="w-full table-fixed border-collapse text-left">
			<thead>
				<tr class="border-b border-glass-edge text-xs font-bold text-ink-muted">
					<th scope="col" class="px-4 py-3">{t('services_col_name')}</th>
					<th scope="col" class="w-28 px-4 py-3">{t('services_col_duration')}</th>
					<th scope="col" class="w-32 px-4 py-3 text-right">{t('services_col_price')}</th>
					<th scope="col" class="w-40 px-4 py-3"
						><span class="sr-only">{t('quote_col_count')}</span></th
					>
				</tr>
			</thead>
			<tbody>
				{#each services as service (service.id)}
					{@const count = countOf(service.id)}
					<tr class="border-b border-glass-edge last:border-b-0">
						<th scope="row" class="px-4 py-2 text-base font-bold text-ink">{service.name}</th>
						<td class="px-4 py-2 text-sm text-ink-muted"
							>{t('service_minutes', { n: service.duration_min })}</td
						>
						<td class="px-4 py-2 text-right text-sm text-ink-muted">
							{service.price_cents === null
								? t('services_price_unset')
								: formatMoney(service.price_cents, settings.currency, settings.locale)}
						</td>
						<td class="px-4 py-2">
							<div class="flex items-center justify-end gap-1">
								{#if count > 0}
									<IconButton
										variant="accent"
										icon="minus"
										label={t('appointment_service_remove', { name: service.name })}
										onclick={() => {
											setLines(removeService(lines, service.id));
										}}
									/>
									<span class="min-w-6 text-center text-base font-bold text-ink">{count}</span>
								{/if}
								<IconButton
									variant="accent"
									icon="plus"
									label={t('quote_service_add', { name: service.name })}
									onclick={() => {
										setLines(tapService(lines, service.id));
									}}
								/>
							</div>
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</Card>
{/snippet}

{#snippet noServices()}
	<Card>
		<EmptyState
			title={t('appointment_services_empty_title')}
			body={t('appointment_services_empty_body')}
			actionLabel={t('appointment_services_empty_action')}
			href={resolve('/servicios/catalogo')}
		/>
	</Card>
{/snippet}

<!-- A message for screen readers only: the copy needs no toast and no change on screen. -->
<span class="sr-only" aria-live="polite">{copied ? t('quote_copied') : ''}</span>

{#if services.length === 0 && !wide}
	{@render noServices()}
{:else if wide}
	<div class="flex items-start gap-8">
		<div class="min-w-0 flex-1">
			{#if services.length === 0}
				{@render noServices()}
			{:else}
				{@render table()}
			{/if}
		</div>
		<Card variant="strong" radius="xl" pad="md" class="flex w-100 shrink-0 flex-col gap-4">
			<h2 class="text-xl font-bold text-ink">{t('quote_heading')}</h2>
			{@render notice()}
			{#if visible.length === 0}
				<p class="text-base text-ink-muted">{t('quote_empty')}</p>
			{:else}
				<p class="text-base whitespace-pre-line text-ink select-text">{text}</p>
				{@render copyButton()}
				{@render startOverButton()}
			{/if}
		</Card>
	</div>
{:else}
	<div class="flex flex-col gap-4">
		<ServicePicker {services} bind:lines={() => lines, setLines} />
		{@render notice()}
		{#if visible.length === 0}
			<p class="px-2 text-base text-ink-muted">{t('quote_empty')}</p>
		{:else}
			<Card as="p" pad="md" class="text-base whitespace-pre-line text-ink select-text">{text}</Card>
			{@render copyButton()}
			{@render startOverButton()}
		{/if}
	</div>
{/if}
