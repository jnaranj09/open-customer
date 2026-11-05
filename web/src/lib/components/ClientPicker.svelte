<script lang="ts">
	import { api } from '../api/app';
	import { errorText, type ApiError } from '../api/errors';
	import type { Client } from '../api/schemas';
	import { t } from '../i18n';
	import { sharingNames } from '../clients';
	import { search } from '../search';
	import { submitOnce } from '../submit';
	import BottomSheet from './BottomSheet.svelte';
	import Button from './Button.svelte';
	import DuplicatePhone from './DuplicatePhone.svelte';
	import TextField from './TextField.svelte';

	// Picks the client of a booking (CUJ 3 step 2): type to search the cached clients (the one
	// search, `lib/search.ts`), or add a new one from the same screen. A client added here is kept
	// in `added` until the next refresh brings it into `clients`.
	let {
		clients,
		value = $bindable(),
		error,
		oncreated
	}: {
		clients: Client[];
		value: string | null;
		error?: string | undefined;
		/** Told after a client is added, so the cache can refresh. */
		oncreated?: (client: Client) => void;
	} = $props();

	let query = $state('');
	let added = $state<Client[]>([]);
	const pool = $derived([...clients, ...added.filter((a) => !clients.some((c) => c.id === a.id))]);
	const results = $derived(search(pool, query));
	const selected = $derived(pool.find((c) => c.id === value) ?? null);

	let adding = $state(false);
	let name = $state('');
	let phone = $state('');
	let failure = $state<ApiError | null>(null);
	let busy = $state(false);

	const sharing = $derived(sharingNames(failure));

	function openAdd(): void {
		name = query.trim();
		phone = '';
		failure = null;
		adding = true;
	}

	const save = submitOnce(
		async (allowDuplicate: boolean) => {
			failure = null;
			const trimmed = phone.trim();
			const result = await api.createClient({
				name: name.trim(),
				...(trimmed === '' ? {} : { phone: trimmed }),
				...(allowDuplicate ? { allow_duplicate_phone: true } : {})
			});
			if (!result.ok) {
				failure = result.error;
				return;
			}
			added = [...added, result.data];
			value = result.data.id;
			query = '';
			adding = false;
			oncreated?.(result.data);
		},
		(pending) => {
			busy = pending;
		}
	);
</script>

<div class="flex flex-col gap-1.5">
	{#if selected !== null}
		<span class="text-sm font-bold text-ink">{t('appointment_client')}</span>
		<div
			class="glass-card flex min-h-tap items-center gap-3 rounded-sm py-1 pr-1 pl-4 text-base text-ink"
		>
			<span class="min-w-0 flex-1 truncate font-bold">{selected.name}</span>
			<button
				type="button"
				class="min-h-tap rounded-pill px-4 text-sm font-bold text-accent-700 hover:bg-accent-100"
				onclick={() => {
					value = null;
				}}
			>
				{t('client_change')}
			</button>
		</div>
	{:else}
		<TextField
			label={t('appointment_client')}
			type="search"
			placeholder={t('client_search_placeholder')}
			autocomplete="off"
			bind:value={query}
			{error}
		/>
		{#if query.trim() !== ''}
			<ul class="glass-card flex flex-col overflow-hidden rounded-sm">
				{#each results as client (client.id)}
					<li>
						<button
							type="button"
							class="flex min-h-tap w-full items-center px-4 text-left text-base text-ink hover:bg-accent-100"
							onclick={() => {
								value = client.id;
								query = '';
							}}
						>
							{client.name}
						</button>
					</li>
				{:else}
					<li class="px-4 py-3 text-sm text-ink-muted">{t('client_no_results')}</li>
				{/each}
			</ul>
		{/if}
		<button
			type="button"
			class="min-h-tap self-start rounded-pill px-4 text-sm font-bold text-accent-700 hover:bg-accent-100"
			onclick={openAdd}
		>
			{t('client_add')}
		</button>
	{/if}
</div>

<BottomSheet
	open={adding}
	title={t('client_add_title')}
	onclose={() => {
		adding = false;
	}}
>
	<TextField label={t('client_name')} autocomplete="off" bind:value={name} />
	<TextField label={t('client_phone')} type="tel" autocomplete="off" bind:value={phone} />
	{#if sharing !== null}
		<DuplicatePhone
			names={sharing}
			{busy}
			label={t('client_save_anyway')}
			onconfirm={() => {
				void save.run(true);
			}}
		/>
	{:else}
		{#if failure !== null}
			<p role="alert" class="text-sm text-error">{errorText(failure)}</p>
		{/if}
		<Button
			variant="primary"
			{busy}
			onclick={() => {
				void save.run(false);
			}}
		>
			{t('client_save')}
		</Button>
	{/if}
</BottomSheet>
