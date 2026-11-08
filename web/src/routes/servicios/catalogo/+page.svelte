<script lang="ts">
	import Card from '../../../lib/components/ui/Card.svelte';
	import type { Service } from '../../../lib/api/schemas';
	import { cache } from '../../../lib/cache/store.svelte';
	import EmptyState from '../../../lib/components/ui/EmptyState.svelte';
	import Icon from '../../../lib/components/icons/Icon.svelte';
	import IconButton from '../../../lib/components/ui/IconButton.svelte';
	import ListRow from '../../../lib/components/ui/ListRow.svelte';
	import ServiceSheet from '../../../lib/components/ServiceSheet.svelte';
	import { t, type MessageKey } from '../../../lib/i18n';
	import { layout } from '../../../lib/layout';
	import { formatMoney } from '../../../lib/money';
	import { serviceSubtitle } from '../../../lib/services';
	import { toaster } from '../../../lib/toaster.svelte';
	import WriteButton from '../../../lib/components/WriteButton.svelte';
	import { useHeaderAction } from '../header.svelte';

	// The Mis servicios tab of Servicios (CUJ 2 step 1, quick-quotes design section 5): the list on
	// the phone and a table on wide, with a sheet to add a service or edit the one she taps. Archived
	// services are listed apart, so she can restore one. "Nuevo servicio" sits in the layout's
	// header slot on wide and is the floating "+" of Hoy on the phone.

	const data = $derived(cache.snapshot);
	const active = $derived(data?.services.filter((s) => !s.archived) ?? []);
	const archived = $derived(data?.services.filter((s) => s.archived) ?? []);
	const wide = $derived(layout.current === 'wide');

	// The service in the sheet; 'new' is the empty form.
	let editing = $state<Service | 'new' | null>(null);
	// Wide: the archived table, collapsed until she opens it.
	let showArchived = $state(false);

	function done(message: MessageKey): void {
		editing = null;
		toaster.say(message);
	}

	useHeaderAction(newService, () => wide && data !== null);
</script>

{#snippet newService()}
	<WriteButton
		variant="strong"
		onclick={() => {
			editing = 'new';
		}}>{t('services_new')}</WriteButton
	>
{/snippet}

{#snippet dot(color: string)}
	<span
		class="size-5 shrink-0 rounded-pill border border-glass-edge"
		style:background-color={color}
		aria-hidden="true"
	></span>
{/snippet}

{#snippet rows(list: Service[], currency: string, locale: string)}
	<Card as="ul" pad="list" class="flex flex-col gap-1">
		{#each list as service (service.id)}
			<li>
				<ListRow
					title={service.name}
					subtitle={serviceSubtitle(service, (minor) => formatMoney(minor, currency, locale))}
					onclick={() => {
						editing = service;
					}}
				>
					{#snippet trailing()}
						{@render dot(service.color)}
					{/snippet}
				</ListRow>
			</li>
		{/each}
	</Card>
{/snippet}

<!-- The wide table: the name is the button that opens the sheet, stretched over the whole row. -->
{#snippet table(list: Service[], currency: string, locale: string)}
	<Card pad="list">
		<table class="w-full border-collapse text-left">
			<thead>
				<tr class="border-b border-glass-edge text-xs font-bold text-ink-muted">
					<th scope="col" class="px-4 py-3">{t('services_col_name')}</th>
					<th scope="col" class="px-4 py-3">{t('services_col_duration')}</th>
					<th scope="col" class="px-4 py-3 text-right">{t('services_col_price')}</th>
					<th scope="col" class="px-4 py-3 text-center">{t('services_col_color')}</th>
				</tr>
			</thead>
			<tbody>
				{#each list as service (service.id)}
					<tr class="relative border-b border-glass-edge last:border-b-0 hover:bg-accent-100">
						<th scope="row" class="px-4 py-3 text-base font-bold text-ink">
							<button
								type="button"
								class="min-h-tap text-left after:absolute after:inset-0"
								onclick={() => {
									editing = service;
								}}>{service.name}</button
							>
						</th>
						<td class="px-4 py-3 text-sm text-ink-muted"
							>{t('service_minutes', { n: service.duration_min })}</td
						>
						<td class="px-4 py-3 text-right text-sm text-ink-muted">
							{service.price_cents === null
								? t('services_price_unset')
								: formatMoney(service.price_cents, currency, locale)}
						</td>
						<td class="px-4 py-3"
							><div class="flex justify-center">{@render dot(service.color)}</div></td
						>
					</tr>
				{/each}
			</tbody>
		</table>
	</Card>
{/snippet}

{#if data === null}
	<p role="status" class="px-2 text-base text-ink-muted">{t('services_loading')}</p>
{:else}
	{#if data.services.length === 0}
		<Card>
			<EmptyState
				title={t('appointment_services_empty_title')}
				body={t('appointment_services_empty_body')}
			/>
		</Card>
	{:else if wide}
		{@render table(active, data.settings.currency, data.settings.locale)}
		{#if archived.length > 0}
			<div class="flex flex-col gap-4">
				<button
					type="button"
					class="flex min-h-tap items-center gap-2 self-start px-4 text-sm font-bold text-ink-muted"
					aria-expanded={showArchived}
					onclick={() => {
						showArchived = !showArchived;
					}}
				>
					<Icon name={showArchived ? 'chevron-down' : 'chevron-right'} class="size-4.5" />
					{t('services_archived_toggle', { count: archived.length })}
				</button>
				{#if showArchived}
					{@render table(archived, data.settings.currency, data.settings.locale)}
				{/if}
			</div>
		{/if}
	{:else}
		{@render rows(active, data.settings.currency, data.settings.locale)}
		{#if archived.length > 0}
			<h2 class="px-2 text-sm font-bold text-ink-muted">{t('archived_heading_masculine')}</h2>
			{@render rows(archived, data.settings.currency, data.settings.locale)}
		{/if}
	{/if}
	{#if !wide}
		<!-- The floating "+", as on Hoy; offline it fades, the offline banner says why. -->
		<div class="fixed right-4 bottom-fab z-20">
			<IconButton
				variant="add"
				icon="plus"
				label={t('services_new')}
				inert={cache.offline}
				onclick={() => {
					editing = 'new';
				}}
			/>
		</div>
	{/if}
	{#if editing !== null}
		<ServiceSheet
			{...editing === 'new' ? {} : { service: editing }}
			currency={data.settings.currency}
			locale={data.settings.locale}
			accent={data.settings.accent_color}
			onclose={() => {
				editing = null;
			}}
			ondone={done}
		/>
	{/if}
{/if}
