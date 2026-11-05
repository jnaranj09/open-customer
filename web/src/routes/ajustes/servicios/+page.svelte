<script lang="ts">
	import { getContext } from 'svelte';
	import type { Service } from '../../../lib/api/schemas';
	import { cache } from '../../../lib/cache/store.svelte';
	import EmptyState from '../../../lib/components/EmptyState.svelte';
	import ListRow from '../../../lib/components/ListRow.svelte';
	import ServiceSheet from '../../../lib/components/ServiceSheet.svelte';
	import { t, type MessageKey } from '../../../lib/i18n';
	import { formatMoney } from '../../../lib/money';
	import { serviceSubtitle } from '../../../lib/services';
	import type { createToaster } from '../../../lib/toaster.svelte';
	import WriteButton from '../../../lib/components/WriteButton.svelte';

	// Ajustes → Servicios (design doc item 77, CUJ 2 step 1): the list, with a sheet to add one or
	// edit the one she taps. Archived services are listed apart, so she can restore one.
	const toaster = getContext<ReturnType<typeof createToaster>>('settings-toaster');

	const data = $derived(cache.snapshot);
	const active = $derived(data?.services.filter((s) => !s.archived) ?? []);
	const archived = $derived(data?.services.filter((s) => s.archived) ?? []);

	// The service in the sheet; 'new' is the empty form.
	let editing = $state<Service | 'new' | null>(null);

	function done(message: MessageKey): void {
		editing = null;
		toaster.say(message);
	}
</script>

{#snippet rows(list: Service[], currency: string, locale: string)}
	<ul class="glass-card flex flex-col gap-1 rounded-lg p-2">
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
						<span
							class="size-5 shrink-0 rounded-pill border border-glass-edge"
							style:background-color={service.color}
							aria-hidden="true"
						></span>
					{/snippet}
				</ListRow>
			</li>
		{/each}
	</ul>
{/snippet}

<div class="flex max-w-190 flex-col gap-4">
	<header class="flex min-h-12 items-center justify-between gap-3">
		<h1 class="text-3xl font-extrabold text-ink">{t('services_title')}</h1>
		{#if data !== null}
			<div class="w-44 shrink-0">
				<WriteButton
					variant="primary"
					onclick={() => {
						editing = 'new';
					}}>{t('services_new')}</WriteButton
				>
			</div>
		{/if}
	</header>

	{#if data === null}
		<p role="status" class="px-2 text-base text-ink-muted">{t('services_loading')}</p>
	{:else}
		{#if data.services.length === 0}
			<div class="glass-card rounded-lg">
				<EmptyState
					title={t('appointment_services_empty_title')}
					body={t('appointment_services_empty_body')}
				/>
			</div>
		{:else}
			{@render rows(active, data.settings.currency, data.settings.locale)}
			{#if archived.length > 0}
				<h2 class="px-2 text-sm font-bold text-ink-muted">{t('archived_heading_masculine')}</h2>
				{@render rows(archived, data.settings.currency, data.settings.locale)}
			{/if}
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
</div>
