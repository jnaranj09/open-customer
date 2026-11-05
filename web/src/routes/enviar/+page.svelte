<script lang="ts">
	import { untrack } from 'svelte';
	import { api } from '../../lib/api/app';
	import { errorText, type ApiError } from '../../lib/api/errors';
	import type { Appointment, DueItem, LogItem } from '../../lib/api/schemas';
	import { appBadge } from '../../lib/cache/refresh';
	import { cache } from '../../lib/cache/store.svelte';
	import { clock } from '../../lib/clock.svelte';
	import DueItemDetail from '../../lib/components/DueItemDetail.svelte';
	import DueItemRow from '../../lib/components/DueItemRow.svelte';
	import NoRulesState from '../../lib/components/NoRulesState.svelte';
	import OverlapSheet from '../../lib/components/OverlapSheet.svelte';
	import Toast from '../../lib/components/Toast.svelte';
	import { addToBooking, groupDue, nextAfter } from '../../lib/due';
	import { t } from '../../lib/i18n';
	import { layout } from '../../lib/layout';
	import { skipItem, startSend, type Sendable } from '../../lib/pending';
	import { submitOnce } from '../../lib/submit';
	import { formatTime, systemNow } from '../../lib/time';
	import { UNDO_MS, delayed } from '../../lib/undo';

	const zone = $derived(
		cache.snapshot?.settings.timezone ?? Intl.DateTimeFormat().resolvedOptions().timeZone
	);
	const today = $derived(clock.today(zone));
	const hasRules = $derived(
		cache.snapshot === null || cache.snapshot.rules.some((r) => !r.archived)
	);

	// An "Omitir" waits UNDO_MS for "Deshacer" before its POST; the item is out of the list meanwhile.
	let skipping = $state<{
		id: string;
		name: string;
		undo: () => void;
		flush: () => Promise<unknown>;
	} | null>(null);

	// The shown due list (the store's `shownDue`), without the item an "Omitir" is waiting on.
	const items = $derived(cache.shownDue.filter((i) => i.id !== skipping?.id));
	const groups = $derived(groupDue(items));
	const ordered = $derived([...groups.late, ...groups.today]);

	// Wide: the item on the right. After a send or skip the item leaves the list and the one that
	// followed it takes the pane; with nothing chosen it is the first item.
	let selectedId = $state<string | null>(null);
	let successorId = $state<string | null>(null);
	const selected = $derived(
		ordered.find((i) => i.id === selectedId) ??
			ordered.find((i) => i.id === successorId) ??
			ordered[0] ??
			null
	);

	function select(id: string): void {
		selectedId = id;
		successorId = nextAfter(ordered, id)?.id ?? null;
	}

	let busy = $state(false);
	let actionError = $state<string | null>(null);
	const setBusy = (pending: boolean): void => {
		busy = pending;
	};

	// The link opens in a new tab on a wide screen, so the app stays; on a phone it opens in this
	// window, which the installed app hands to WhatsApp. The confirmation was stored before.
	function openLink(url: string): void {
		if (layout.current === 'wide') {
			window.open(url, '_blank', 'noopener');
		} else {
			window.location.assign(url);
		}
	}

	const send = submitOnce(async (item: Sendable) => {
		actionError = null;
		select(item.id);
		// On a phone the link unloads this page, which would lose a skip still waiting for its
		// "Deshacer": send it now and wait for it. Its failure shows through the skip's own handler.
		await skipping?.flush().catch(() => undefined);
		const started = await startSend(item, openLink, { store: cache, now: systemNow });
		if (!started.ok) {
			actionError = errorText(started.error);
		}
	}, setBusy);

	function skip(item: DueItem): void {
		actionError = null;
		select(item.id);
		let failure: ApiError | null = null;
		const pending = delayed(async () => {
			const result = await skipItem(item, {
				api,
				badge: appBadge,
				store: cache,
				now: systemNow,
				due: cache.shownDue
			});
			if (result.ok) {
				await cache.refresh();
			} else {
				failure = result.error;
			}
		}, UNDO_MS);
		skipping = {
			id: item.id,
			name: item.client_name,
			undo: () => {
				pending.cancel();
			},
			flush: () => pending.flush()
		};
		pending.done.then(
			() => {
				skipping = null;
				if (failure !== null) {
					actionError = errorText(failure);
				}
			},
			() => {
				console.warn('code=skip_failed');
				skipping = null;
				actionError = t('error_internal');
			}
		);
	}

	// "Agregar a la cita": the future appointment gets the return service; a 409 asks first.
	let overlap = $state<{ item: DueItem; appointments: Appointment[] } | null>(null);

	const addToAppointment = submitOnce(async (item: DueItem, allowOverlap: boolean) => {
		const prompt = item.prompt;
		if (prompt === null) {
			return;
		}
		actionError = null;
		select(item.id);
		const result = await addToBooking(api, prompt, allowOverlap);
		if (result.ok) {
			overlap = null;
			await cache.refresh();
		} else if (result.error.kind === 'overlap') {
			overlap = { item, appointments: result.error.overlapping };
		} else {
			overlap = null;
			actionError = errorText(result.error);
		}
	}, setBusy);

	// Today's sent items come from the server (the log is not cached), so they need a connection.
	let sent = $state<LogItem[] | null>(null);
	let sentError = $state<ApiError | null>(null);
	let sentRun = 0;

	function loadSent(day: string): void {
		const run = ++sentRun;
		void api.listReminderLog({ date: day }).then((result) => {
			if (run !== sentRun) {
				return;
			}
			// A skipped item is not a sent message: only `sent` rows are listed.
			sent = result.ok ? result.data.filter((row) => row.action === 'sent') : null;
			sentError = result.ok ? null : result.error;
		});
	}

	// Syncs with the server: again after each refresh, so a send shows up under "Enviados".
	const sentKey = $derived([today, cache.meta.lastSyncAt ?? ''] as const);
	$effect(() => {
		const [day] = sentKey;
		if (cache.offline) {
			return;
		}
		untrack(() => {
			loadSent(day);
		});
	});

	const sentOffline = $derived(cache.offline || sentError?.kind === 'offline');
	const sentFailure = $derived(
		sentError !== null && sentError.kind !== 'offline' ? errorText(sentError) : null
	);
	const count = $derived(
		items.length === 1 ? t('due_count_one') : t('due_count_other', { count: items.length })
	);
</script>

{#snippet entry(item: DueItem)}
	{#if layout.current === 'wide'}
		<li>
			<DueItemRow
				title={item.client_name}
				subtitle={item.text}
				selected={selected?.id === item.id}
				onselect={() => {
					select(item.id);
				}}
			/>
		</li>
	{:else}
		<li class="glass-card rounded-lg p-5">
			{@render detail(item)}
		</li>
	{/if}
{/snippet}

{#snippet detail(item: DueItem)}
	<DueItemDetail
		{item}
		{zone}
		offline={cache.offline}
		{busy}
		skipBlocked={skipping !== null}
		onsend={(sendable: Sendable) => void send.run(sendable)}
		onskip={skip}
		onadd={(due: DueItem) => void addToAppointment.run(due, false)}
	/>
{/snippet}

{#snippet group(label: string, list: DueItem[])}
	{#if list.length > 0}
		<section class="flex flex-col gap-3">
			<h2 class="text-sm font-bold text-ink-muted">{label}</h2>
			<ul class="flex flex-col gap-3">
				{#each list as item (item.id)}
					{@render entry(item)}
				{/each}
			</ul>
		</section>
	{/if}
{/snippet}

{#snippet lists()}
	{@render group(t('due_overdue_heading'), groups.late)}
	{@render group(t('today_title'), groups.today)}
	{#if items.length === 0}
		<p class="glass-card rounded-lg px-5 py-4 text-base text-ink">{t('due_today_none')}</p>
	{/if}
	<section class="flex flex-col gap-3">
		<h2 class="text-sm font-bold text-ink-muted">{t('due_sent_heading')}</h2>
		{#if sentOffline}
			<p class="text-sm text-ink-muted">{t('due_sent_offline')}</p>
		{:else if sentFailure !== null}
			<p role="alert" class="text-sm text-error">{sentFailure}</p>
		{:else if sent !== null && sent.length === 0}
			<p class="text-sm text-ink-muted">{t('due_sent_empty')}</p>
		{:else if sent !== null}
			<ul class="flex flex-col gap-3">
				{#each sent as row (row.id)}
					<li>
						<DueItemRow
							title={row.client_name}
							subtitle={row.body}
							meta={formatTime(row.acted_at, zone)}
						/>
					</li>
				{/each}
			</ul>
		{/if}
	</section>
{/snippet}

<div class="flex flex-col gap-4">
	<header class="flex flex-col gap-0.5">
		<h1 class="text-3xl font-extrabold text-ink">{t('due_today_heading')}</h1>
		<p class="text-sm text-ink-muted">{count}</p>
	</header>

	{#if actionError !== null}
		<p role="alert" class="glass-card rounded-lg px-5 py-3 text-base text-error">{actionError}</p>
	{/if}

	{#if !hasRules}
		<NoRulesState />
	{:else if layout.current === 'wide'}
		<div class="flex items-start gap-6">
			<div class="flex w-105 shrink-0 flex-col gap-4">{@render lists()}</div>
			{#if selected !== null}
				<section class="glass-card min-w-0 flex-1 rounded-lg p-6">
					{@render detail(selected)}
				</section>
			{/if}
		</div>
	{:else}
		{@render lists()}
	{/if}
</div>

{#if skipping !== null}
	{#key skipping.id}
		<Toast
			message={t('due_skipped', { name: skipping.name })}
			actionLabel={t('due_undo')}
			onaction={skipping.undo}
		/>
	{/key}
{/if}

<OverlapSheet
	open={overlap !== null}
	overlapping={overlap?.appointments ?? []}
	{zone}
	{busy}
	onconfirm={() => {
		if (overlap !== null) {
			void addToAppointment.run(overlap.item, true);
		}
	}}
	onclose={() => {
		overlap = null;
	}}
/>
