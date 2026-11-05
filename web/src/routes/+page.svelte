<script lang="ts">
	import { resolve } from '$app/paths';
	import { untrack } from 'svelte';
	import { api } from '../lib/api/app';
	import { errorText, type ApiError } from '../lib/api/errors';
	import type { Appointment } from '../lib/api/schemas';
	import { inWindow, pickedValue, viewRange, type AgendaView } from '../lib/agenda';
	import { clock } from '../lib/clock.svelte';
	import { cache } from '../lib/cache/store.svelte';
	import { bootstrapWindow } from '../lib/cache/refresh';
	import Button from '../lib/components/Button.svelte';
	import DayView from '../lib/components/DayView.svelte';
	import DueCard from '../lib/components/DueCard.svelte';
	import Icon from '../lib/components/icons/Icon.svelte';
	import PushActivate from '../lib/components/PushActivate.svelte';
	import WeekView from '../lib/components/WeekView.svelte';
	import { t } from '../lib/i18n';
	import { layout } from '../lib/layout';
	import { shownDay } from '../lib/shownDay.svelte';
	import { addDays, formatDayLong, formatDayShort, formatWeekdayShort, todayIn } from '../lib/time';

	const zone = $derived(
		cache.snapshot?.settings.timezone ?? Intl.DateTimeFormat().resolvedOptions().timeZone
	);
	const today = $derived(clock.today(zone));

	// Read the clock again when the app comes back and after each refresh, so Hoy follows midnight.
	$effect(() => {
		if (cache.meta.lastSyncAt !== null) {
			clock.tick();
		}
	});
	$effect(() => {
		const handler = (): void => {
			if (document.visibilityState === 'visible') {
				clock.tick();
			}
		};
		window.addEventListener('focus', handler);
		document.addEventListener('visibilitychange', handler);
		return () => {
			window.removeEventListener('focus', handler);
			document.removeEventListener('visibilitychange', handler);
		};
	});

	// What she picked; until she picks, the business day and the layout's usual view.
	let pickedDay = $state<string | null>(null);
	let pickedView = $state<AgendaView | null>(null);
	const day = $derived(pickedDay ?? today);
	const view = $derived<AgendaView>(pickedView ?? (layout.current === 'wide' ? 'week' : 'day'));
	const range = $derived(viewRange(view, day));

	// The sidebar's "Nueva cita" opens on the day shown here.
	$effect(() => {
		shownDay.set(day);
		return () => {
			shownDay.set(null);
		};
	});

	// The cache holds the `/bootstrap` window around the day of the last refresh.
	const win = $derived.by(() => {
		const last = cache.meta.lastSyncAt;
		return last === null ? null : bootstrapWindow(todayIn(zone, new Date(last)));
	});
	const cached = $derived(win !== null && inWindow(range.from, win) && inWindow(range.to, win));

	// Days outside the window come from the server, online only.
	let remote = $state<{ key: string; items: Appointment[] } | null>(null);
	let remoteError = $state<ApiError | null>(null);
	const key = $derived(`${range.from}|${range.to}`);
	let askRun = 0;

	function load(ask: { from: string; to: string }): void {
		const asked = `${ask.from}|${ask.to}`;
		const run = ++askRun;
		remoteError = null;
		void api.listAppointments(ask).then((result) => {
			if (run !== askRun) {
				return;
			}
			if (result.ok) {
				remote = { key: asked, items: result.data };
			} else {
				remote = null;
				remoteError = result.error;
			}
		});
	}

	$effect(() => {
		if (cached || cache.offline) {
			return;
		}
		const ask = { from: range.from, to: range.to };
		untrack(() => {
			load(ask);
		});
	});

	const appointments = $derived.by((): Appointment[] | null => {
		if (cached) {
			return cache.snapshot?.appointments ?? [];
		}
		return remote?.key === key ? remote.items : null;
	});
	const needsConnection = $derived(!cached && (cache.offline || remoteError?.kind === 'offline'));
	const loadFailure = $derived(
		!cached && remoteError !== null && remoteError.kind !== 'offline'
			? errorText(remoteError)
			: null
	);

	const dueItems = $derived(cache.snapshot?.due ?? []);
	const hasRules = $derived(
		cache.snapshot === null || cache.snapshot.rules.some((r) => !r.archived)
	);

	const step = $derived(view === 'week' ? 7 : 1);
	const title = $derived.by(() => {
		if (view === 'week') {
			return `${formatDayShort(range.from)} – ${formatDayShort(range.to)}`;
		}
		return day === today ? t('today_title') : formatDayLong(day);
	});
	const caption = $derived(view === 'day' && day === today ? formatDayLong(day) : null);
	// The short line between the arrows on compact; the title is too long for it.
	const switcher = $derived.by(() => {
		if (view === 'week') {
			return title;
		}
		return day === today
			? t('agenda_today_short', { date: formatDayShort(day) })
			: `${formatWeekdayShort(day)} ${formatDayShort(day)}`;
	});

	function pickDay(picked: string): void {
		pickedDay = pickedValue(picked, today);
	}

	function move(by: number): void {
		pickDay(addDays(day, by * step));
	}

	function openDay(picked: string): void {
		pickDay(picked);
		pickedView = 'day';
	}

	function onDateInput(e: Event & { currentTarget: HTMLInputElement }): void {
		if (e.currentTarget.value !== '') {
			pickDay(e.currentTarget.value);
		}
	}

	const VIEWS = [
		{ id: 'day', label: 'view_day' },
		{ id: 'week', label: 'view_week' }
	] as const;
	const newHref = $derived(`/citas/nueva?dia=${day}` as const);
</script>

{#snippet backupLine()}
	{#if cache.meta.backupRed}
		<a
			href={resolve('/ajustes/estado')}
			class="flex min-h-tap items-center gap-2 rounded-md border border-error px-4 text-sm font-bold text-error hover:bg-glass-strong"
		>
			{t('backup_red')}
			<Icon name="chevron-right" class="ml-auto size-4.5" />
		</a>
	{/if}
{/snippet}

<div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:gap-8">
	<div class="flex min-w-0 flex-1 flex-col gap-4">
		{#if layout.current === 'compact'}
			{@render backupLine()}
		{/if}

		<header class="flex flex-col gap-0.5">
			<h1 class="font-display text-3xl font-semibold text-ink">{title}</h1>
			{#if caption !== null}
				<p class="text-sm text-ink-muted">{caption}</p>
			{/if}
		</header>

		{#if layout.current === 'compact'}
			<DueCard items={dueItems} {hasRules} />
			<PushActivate />
		{/if}

		<div class="flex items-center gap-2">
			<button
				type="button"
				class="grid size-11 place-items-center rounded-pill text-ink hover:bg-accent-100"
				title={view === 'week' ? t('agenda_prev_week') : t('agenda_prev_day')}
				aria-label={view === 'week' ? t('agenda_prev_week') : t('agenda_prev_day')}
				onclick={() => {
					move(-1);
				}}
			>
				<Icon name="chevron-left" class="size-5.5" />
			</button>
			<label
				class="relative flex min-h-tap min-w-0 flex-1 items-center rounded-pill px-2 text-base font-bold text-ink hover:bg-accent-100 focus-within:outline-2 lg:flex-none lg:w-44 lg:rounded-sm lg:border lg:border-glass-edge lg:bg-glass-strong lg:px-0"
				title={t('agenda_pick_date')}
			>
				<span class="sr-only">{t('agenda_pick_date')}</span>
				<span class="truncate lg:hidden">{switcher}</span>
				<input
					type="date"
					value={day}
					onchange={onDateInput}
					class="absolute inset-0 size-full opacity-0 lg:static lg:px-3 lg:opacity-100"
				/>
			</label>
			<button
				type="button"
				class="grid size-11 place-items-center rounded-pill text-ink hover:bg-accent-100"
				title={view === 'week' ? t('agenda_next_week') : t('agenda_next_day')}
				aria-label={view === 'week' ? t('agenda_next_week') : t('agenda_next_day')}
				onclick={() => {
					move(1);
				}}
			>
				<Icon name="chevron-right" class="size-5.5" />
			</button>
			<div
				role="group"
				aria-label={t('agenda_view')}
				class="glass-card flex rounded-pill lg:ml-auto"
			>
				{#each VIEWS as v (v.id)}
					<button
						type="button"
						aria-pressed={view === v.id}
						class="min-h-tap rounded-pill px-4 text-sm font-bold {view === v.id
							? 'bg-accent text-on-accent'
							: 'text-ink-muted hover:bg-accent-100'}"
						onclick={() => {
							pickedView = v.id;
						}}
					>
						{t(v.label)}
					</button>
				{/each}
			</div>
		</div>

		{#if needsConnection}
			<p role="status" class="glass-card rounded-lg px-5 py-3 text-base text-ink">
				{t('agenda_needs_connection')}
			</p>
		{:else if loadFailure !== null}
			<div role="alert" class="glass-card flex flex-col gap-3 rounded-lg px-5 py-3">
				<p class="text-base text-error">{loadFailure}</p>
				<Button
					variant="ghost"
					onclick={() => {
						load(range);
					}}
				>
					{t('agenda_retry')}
				</Button>
			</div>
		{:else if appointments === null}
			<p role="status" class="px-2 text-base text-ink-muted">{t('agenda_loading')}</p>
		{:else if view === 'week'}
			<WeekView {day} {appointments} {zone} {today} onpickday={openDay} />
		{:else}
			<DayView {day} {appointments} {zone} {today} />
		{/if}
	</div>

	{#if layout.current === 'wide'}
		<aside class="flex w-90 shrink-0 flex-col gap-4">
			<DueCard items={dueItems} {hasRules} />
			{@render backupLine()}
			<PushActivate />
		</aside>
	{:else}
		<!-- The floating "+" of the mockup; the sidebar's "Nueva cita" does this on wide. -->
		<div class="fixed right-4 bottom-32 z-20 flex flex-col items-end gap-2">
			{#if cache.offline}
				<p class="glass-card rounded-lg px-4 py-2 text-sm text-ink">{t('offline_cannot_save')}</p>
				<span
					class="grid size-14 place-items-center rounded-pill bg-accent text-on-accent opacity-60 shadow-raised"
					role="img"
					aria-label={t('appointment_new')}
				>
					<Icon name="plus" class="size-7" />
				</span>
			{:else}
				<a
					href={resolve(newHref)}
					title={t('appointment_new')}
					aria-label={t('appointment_new')}
					class="grid size-14 place-items-center rounded-pill bg-accent text-on-accent shadow-raised hover:bg-accent-700"
				>
					<Icon name="plus" class="size-7" />
				</a>
			{/if}
		</div>
	{/if}
</div>
