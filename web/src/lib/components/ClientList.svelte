<script lang="ts">
	import { resolve } from '$app/paths';
	import { api } from '../api/app';
	import { errorText, type ApiError } from '../api/errors';
	import type { Client } from '../api/schemas';
	import { cache } from '../cache/store.svelte';
	import { visibleClients } from '../clients';
	import { t } from '../i18n';
	import { submitOnce } from '../submit';
	import Button from './Button.svelte';
	import { buttonClasses } from './buttonStyle';
	import EmptyState from './EmptyState.svelte';
	import ListRow from './ListRow.svelte';
	import TextField from './TextField.svelte';

	// The client list (design doc item 76): the search box over the cached clients, online and
	// offline, and "Ver archivadas", which loads the archived ones (online only) and searches them
	// with the same function. The list state lives here, so it stays while she opens a client.
	let { selectedId }: { selectedId: string | null } = $props();

	let query = $state('');
	let showArchived = $state(false);
	let archived = $state<Client[] | null>(null);
	let archivedError = $state<ApiError | null>(null);
	let loading = $state(false);

	const active = $derived(cache.snapshot?.clients ?? []);
	const pool = $derived(showArchived ? (archived ?? []) : active);
	const rows = $derived(visibleClients(pool, query));

	const toggle = submitOnce(
		async () => {
			if (showArchived) {
				showArchived = false;
				return;
			}
			archivedError = null;
			const result = await api.listClients({ archived: true });
			if (result.ok) {
				archived = result.data;
				showArchived = true;
			} else {
				archivedError = result.error;
			}
		},
		(pending) => {
			loading = pending;
		}
	);

	const failure = $derived(
		archivedError === null
			? null
			: archivedError.kind === 'offline'
				? t('clients_archived_offline')
				: errorText(archivedError)
	);
	const noneAtAll = $derived(
		cache.snapshot !== null && !showArchived && active.length === 0 && query.trim() === ''
	);
</script>

{#snippet archivedBadge()}
	<span class="rounded-pill bg-glass-tint px-3 py-1 text-xs font-bold text-accent-700">
		{t('clients_archived_badge')}
	</span>
{/snippet}

<div class="flex flex-col gap-4">
	<header class="flex items-center gap-3">
		<h1 class="flex-1 font-display text-3xl font-semibold text-ink">{t('nav_clients')}</h1>
		<a href={resolve('/clientas/nueva')} class={buttonClasses('ghost')}>{t('clients_new')}</a>
	</header>

	<TextField
		label={t('client_search')}
		type="search"
		placeholder={t('client_search_placeholder')}
		autocomplete="off"
		bind:value={query}
	/>

	{#if cache.snapshot === null}
		<p role="status" class="px-2 text-base text-ink-muted">{t('clients_loading')}</p>
	{:else if noneAtAll}
		<div class="glass-card rounded-lg">
			<EmptyState
				title={t('clients_empty_title')}
				body={t('clients_empty_body')}
				actionLabel={t('clients_new')}
				href={resolve('/clientas/nueva')}
			/>
		</div>
	{:else}
		<ul class="glass-card flex flex-col divide-y divide-glass-edge rounded-lg px-1 py-1">
			{#each rows as client (client.id)}
				<li>
					<ListRow
						href={resolve('/clientas/[id]', { id: client.id })}
						title={client.name}
						{...client.phone_e164 === null ? {} : { subtitle: client.phone_e164 }}
						selected={client.id === selectedId}
						{...client.archived ? { trailing: archivedBadge } : {}}
					/>
				</li>
			{:else}
				<li class="px-4 py-3 text-sm text-ink-muted">
					{showArchived && query.trim() === ''
						? t('clients_archived_empty')
						: t('client_no_results')}
				</li>
			{/each}
		</ul>
	{/if}

	{#if failure !== null}
		<p role="alert" class="text-sm text-error">{failure}</p>
	{/if}
	<Button
		variant="secondary"
		busy={loading}
		{...cache.offline && !showArchived
			? { disabled: true as const, disabledReason: t('clients_archived_offline') }
			: {}}
		onclick={() => {
			void toggle.run();
		}}
	>
		{showArchived ? t('clients_show_active') : t('clients_show_archived')}
	</Button>
</div>
