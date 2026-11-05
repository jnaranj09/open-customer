<script lang="ts">
	import Callout from '../../../lib/components/ui/Callout.svelte';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { untrack } from 'svelte';
	import { api } from '../../../lib/api/app';
	import { errorText, type ApiError } from '../../../lib/api/errors';
	import type { Appointment, Client, LogItem } from '../../../lib/api/schemas';
	import { clock } from '../../../lib/clock.svelte';
	import { business } from '../../../lib/business.svelte';
	import { showThenFetch } from '../../../lib/cachedLoad';
	import { attrDisplay, STATUS_NAME } from '../../../lib/booking';
	import { bootstrapWindow } from '../../../lib/cache/refresh';
	import { cache } from '../../../lib/cache/store.svelte';
	import ActionButton from '../../../lib/components/ActionButton.svelte';
	import Button from '../../../lib/components/ui/Button.svelte';
	import BottomSheet from '../../../lib/components/ui/BottomSheet.svelte';
	import ClientForm from '../../../lib/components/ClientForm.svelte';
	import DuplicatePhone from '../../../lib/components/DuplicatePhone.svelte';
	import IconButton from '../../../lib/components/ui/IconButton.svelte';
	import Card from '../../../lib/components/ui/Card.svelte';
	import ListRow from '../../../lib/components/ui/ListRow.svelte';
	import LoadFailure from '../../../lib/components/LoadFailure.svelte';
	import PhotoSection from '../../../lib/components/PhotoSection.svelte';
	import Toast from '../../../lib/components/Toast.svelte';
	import { historySplit, sharingNames } from '../../../lib/clients';
	import { t } from '../../../lib/i18n';
	import { formatMoney } from '../../../lib/money';
	import { submitOnce } from '../../../lib/submit';
	import { createToaster } from '../../../lib/toaster.svelte';
	import { formatDayWithYear, formatTime, todayIn } from '../../../lib/time';
	import WriteButton from '../../../lib/components/WriteButton.svelte';

	// One client (design doc item 76): her facts, the appointment history, the photo gallery, the
	// reminders sent to her, edit mode and archive. The server holds the truth: every change waits
	// for its answer, and offline the cached copy is read-only.
	const id = $derived(page.params.id ?? '');
	const data = $derived(cache.snapshot);
	const zone = $derived(business.zone);

	let client = $state<Client | null>(null);
	let loadError = $state<ApiError | null>(null);
	let editing = $state(false);
	const toaster = createToaster();

	// The cached copy shows at once (an archived client is not cached); the server's answer replaces it.
	async function load(want: string): Promise<void> {
		loadError = null;
		const failure = await showThenFetch(
			untrack(() => cache.snapshot?.clients.find((c) => c.id === want)),
			() => api.getClient(want),
			() => want === id,
			(value) => {
				client = value;
			}
		);
		if (failure !== undefined) {
			loadError = failure;
		}
	}

	// Syncs with the server: each client opened reads its own data again.
	$effect(() => {
		const want = id;
		untrack(() => {
			editing = false;
			everything = null;
			historyError = null;
			reminders = null;
			remindersError = null;
			actionError = null;
			void load(want);
			void loadReminders(want);
		});
	});

	const defs = $derived((data?.attributes ?? []).filter((a) => a.scope === 'client'));
	const facts = $derived.by(() => {
		const c = client;
		if (c === null) {
			return [];
		}
		return defs
			.map((d) => ({ label: d.label, text: attrDisplay(d, c.attrs[d.key]) }))
			.filter((d): d is { label: string; text: string } => d.text !== null);
	});

	const dayLabel = (iso: string): string =>
		formatDayWithYear(todayIn(zone, new Date(iso)), clock.today(zone));

	// History: the cached window first; the rest from the server on request.
	const cacheWindow = $derived(
		bootstrapWindow(todayIn(zone, new Date(cache.meta.lastSyncAt ?? Date.now())))
	);
	const newestFirst = (a: Appointment, b: Appointment): number =>
		Date.parse(b.starts_at) - Date.parse(a.starts_at);
	const cachedAppts = $derived(
		(data?.appointments ?? []).filter((a) => a.client_id === id).sort(newestFirst)
	);
	let everything = $state<Appointment[] | null>(null);
	let historyError = $state<string | null>(null);
	let loadingAll = $state(false);
	const others = $derived(
		everything === null
			? []
			: historySplit(everything, cacheWindow, zone).needsServer.sort(newestFirst)
	);

	const loadAll = submitOnce(
		async () => {
			historyError = null;
			const result = await api.listClientAppointments(id);
			if (result.ok) {
				everything = result.data;
			} else {
				historyError =
					result.error.kind === 'offline'
						? t('client_history_more_offline')
						: errorText(result.error);
			}
		},
		(pending) => {
			loadingAll = pending;
		}
	);

	// Reminders sent to her: the log is not cached, so it needs a connection.
	let reminders = $state<LogItem[] | null>(null);
	let remindersError = $state<string | null>(null);

	async function loadReminders(want: string): Promise<void> {
		const result = await api.listReminderLog({ client_id: want });
		if (want !== id) {
			return;
		}
		if (result.ok) {
			reminders = result.data;
			remindersError = null;
		} else {
			remindersError =
				result.error.kind === 'offline' ? t('client_reminders_offline') : errorText(result.error);
		}
	}

	// Archive and restore
	let asking = $state(false);
	let actionError = $state<ApiError | null>(null);
	let busy = $state(false);
	const sharing = $derived(sharingNames(actionError));

	const setArchived = submitOnce(
		async (archived: boolean, allowDuplicate: boolean) => {
			if (client === null) {
				return;
			}
			actionError = null;
			const result = await api.updateClient(client.id, {
				archived,
				...(allowDuplicate ? { allow_duplicate_phone: true } : {})
			});
			if (!result.ok) {
				actionError = result.error;
				return;
			}
			client = result.data;
			asking = false;
			toaster.say(archived ? 'client_archived' : 'client_restored');
			void cache.refresh();
		},
		(pending) => {
			busy = pending;
		}
	);
</script>

{#snippet appointmentRow(a: Appointment)}
	<li>
		<ListRow
			href={resolve('/citas/[id]', { id: a.id })}
			title="{dayLabel(a.starts_at)} · {formatTime(a.starts_at, zone)}"
			subtitle={[
				a.services.map((s) => s.name).join(', '),
				formatMoney(a.total_cents, business.currency, business.locale),
				t(STATUS_NAME[a.status])
			]
				.filter((x) => x !== '')
				.join(' · ')}
		/>
	</li>
{/snippet}

<div class="flex flex-col gap-4">
	<header class="flex items-center gap-2">
		<IconButton
			variant="plain"
			icon="chevron-left"
			label={t('client_back')}
			href={resolve('/clientas')}
			class="lg:hidden"
		/>
		<h1 class="text-3xl font-extrabold text-ink">
			{editing ? t('client_edit_title') : (client?.name ?? '')}
		</h1>
	</header>

	{#if loadError !== null}
		<LoadFailure
			error={loadError}
			onretry={() => {
				void load(id);
			}}
		/>
	{:else if client === null || data === null}
		<p role="status" class="px-2 text-base text-ink-muted">{t('client_loading')}</p>
	{:else if editing}
		<ClientForm
			{data}
			{client}
			onsaved={(saved: Client) => {
				client = saved;
				editing = false;
				toaster.say('client_saved');
			}}
			oncancel={() => {
				editing = false;
			}}
		/>
	{:else}
		<div class="flex flex-col gap-4 lg:max-w-190">
			<Card as="section" radius="xl" pad="section" class="flex flex-col gap-4">
				{#if client.archived}
					<Callout as="p" class="text-sm font-bold">
						{t('client_archived_note')}
					</Callout>
				{/if}
				<dl class="flex flex-col gap-1 text-base">
					{#if client.phone_e164 !== null}
						<div class="flex gap-2">
							<dt class="font-bold text-ink">{t('client_phone')}:</dt>
							<dd class="text-ink">{client.phone_e164}</dd>
						</div>
					{/if}
					{#each facts as fact (fact.label)}
						<div class="flex gap-2">
							<dt class="font-bold text-ink">{fact.label}:</dt>
							<dd class="text-ink">{fact.text}</dd>
						</div>
					{/each}
					{#if client.notes !== null}
						<div class="flex gap-2">
							<dt class="font-bold text-ink">{t('client_notes')}:</dt>
							<dd class="text-ink">{client.notes}</dd>
						</div>
					{/if}
				</dl>

				<div class="flex flex-wrap items-start gap-3">
					<ActionButton
						variant="ghost"
						onclick={() => {
							editing = true;
						}}
					>
						{t('client_edit')}
					</ActionButton>
					{#if client.archived}
						<ActionButton
							variant="glass"
							{busy}
							onclick={() => {
								void setArchived.run(false, false);
							}}
						>
							{t('client_restore')}
						</ActionButton>
					{:else}
						<ActionButton
							variant="glass"
							onclick={() => {
								actionError = null;
								asking = true;
							}}
						>
							{t('client_archive')}
						</ActionButton>
					{/if}
				</div>
				{#if client.archived}
					{#if sharing !== null}
						<DuplicatePhone
							names={sharing}
							{busy}
							label={t('client_restore_anyway')}
							onconfirm={() => {
								void setArchived.run(false, true);
							}}
						/>
					{:else if actionError !== null}
						<p role="alert" class="text-sm text-error">{errorText(actionError)}</p>
					{/if}
				{/if}
			</Card>

			<Card as="section" radius="xl" pad="section" class="flex flex-col gap-3">
				<h2 class="text-xl font-bold text-ink">{t('client_history')}</h2>
				<ul class="flex flex-col divide-y divide-glass-edge">
					{#each cachedAppts as a (a.id)}
						{@render appointmentRow(a)}
					{:else}
						{#if everything === null}
							<li class="text-sm text-ink-muted">{t('client_history_empty')}</li>
						{/if}
					{/each}
				</ul>
				{#if others.length > 0}
					<h3 class="text-base font-bold text-ink">{t('client_history_older')}</h3>
					<ul class="flex flex-col divide-y divide-glass-edge">
						{#each others as a (a.id)}
							{@render appointmentRow(a)}
						{/each}
					</ul>
				{/if}
				{#if historyError !== null}
					<p role="alert" class="text-sm text-error">{historyError}</p>
				{/if}
				{#if everything === null}
					<Button
						variant="glass"
						busy={loadingAll}
						{...cache.offline
							? { disabled: true as const, disabledReason: t('client_history_more_offline') }
							: {}}
						onclick={() => {
							void loadAll.run();
						}}
					>
						{t('client_history_more')}
					</Button>
				{/if}
			</Card>

			{#key client.id}
				<PhotoSection clientId={client.id} empty="photos_empty_client" />
			{/key}

			<Card as="section" radius="xl" pad="section" class="flex flex-col gap-3">
				<h2 class="text-xl font-bold text-ink">{t('client_reminders')}</h2>
				{#if remindersError !== null}
					<p role="alert" class="text-sm text-error">{remindersError}</p>
				{:else if reminders !== null && reminders.length === 0}
					<p class="text-sm text-ink-muted">{t('client_reminders_empty')}</p>
				{:else if reminders !== null}
					<ul class="flex flex-col divide-y divide-glass-edge">
						{#each reminders as row (row.id)}
							<li class="flex flex-col gap-0.5 py-3">
								<span class="text-sm font-bold text-ink">
									{row.action === 'sent' ? t('reminder_sent') : t('reminder_skipped')} · {dayLabel(
										row.acted_at
									)}
									· {formatTime(row.acted_at, zone)}
								</span>
								{#if row.action === 'sent' && row.body}
									<span class="text-sm text-ink-muted">{row.body}</span>
								{/if}
							</li>
						{/each}
					</ul>
				{/if}
			</Card>
		</div>
	{/if}
</div>

<BottomSheet
	open={asking}
	title={t('client_archive')}
	onclose={() => {
		asking = false;
	}}
>
	<p class="text-base text-ink">{t('client_archive_body')}</p>
	{#if actionError !== null}
		<p role="alert" class="text-sm text-error">{errorText(actionError)}</p>
	{/if}
	<WriteButton
		variant="danger"
		{busy}
		onclick={() => {
			void setArchived.run(true, false);
		}}
	>
		{t('client_archive')}
	</WriteButton>
	<Button
		variant="glass"
		onclick={() => {
			asking = false;
		}}
	>
		{t('cancel')}
	</Button>
</BottomSheet>

{#if toaster.current !== null}
	{#key toaster.current.n}
		<Toast message={toaster.current.message} />
	{/key}
{/if}
