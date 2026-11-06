<script lang="ts">
	import Card from './ui/Card.svelte';
	import { resolve } from '$app/paths';
	import type { Appointment } from '../api/schemas';
	import { layoutDay, scrollHour, type Placed } from '../agenda';
	import { t } from '../i18n';
	import { formatDayShort, formatTime, formatWeekdayShort } from '../time';

	let {
		days,
		appointments,
		zone,
		today,
		onpickday
	}: {
		/** The days to show, one column each. */
		days: string[];
		appointments: Appointment[];
		zone: string;
		today: string;
		/** Given, each column has a head (weekday and date) that opens that day: the week. */
		onpickday?: (day: string) => void;
	} = $props();

	const byId = $derived(new Map(appointments.map((a) => [a.id, a])));
	const lanes = $derived(days.map((day) => ({ day, placed: layoutDay(appointments, zone, day) })));
	// The whole day, in its own scroll area: there is no business-hours setting, so no hour is
	// hidden. It opens at the earliest appointment that starts on the shown days, or 08:00.
	const hours = Array.from({ length: 24 }, (_, i) => i);
	const startHour = $derived(scrollHour(lanes.flatMap((l) => l.placed)));
	const dayKey = $derived(days.join());
	const minutes = (p: Placed): { top: number; height: number } => ({
		top: p.startMin,
		height: p.endMin - p.startMin
	});
	// At most this long, a box (32 px at 4rem an hour) has room for one line, not three: the time
	// and the name share it. Every box is at least a 44 px tap target, so a short one overlaps the
	// slot below a little; the later appointment is drawn on top.
	const ONE_LINE_MIN = 30;

	let scroller = $state<HTMLDivElement>();
	// Scrolls to the start hour when the shown days or that hour change: a sync with the DOM's
	// scroll position, which is not app state. The week's sticky heads are kept clear.
	$effect(() => {
		const el = scroller;
		const line = el?.querySelector<HTMLElement>(`[data-hour="${String(startHour)}"]`);
		// Reading `dayKey` makes a new day or week scroll again, even with the same start hour.
		if (el === undefined || line == null || dayKey === '') {
			return;
		}
		const head = el.querySelector<HTMLElement>('[data-head]')?.offsetHeight ?? 0;
		el.scrollTop += line.getBoundingClientRect().top - el.getBoundingClientRect().top - head - 16;
	});

	const STATUS_EDGE = {
		scheduled: 'border-accent',
		confirmed: 'border-success',
		cancelled: 'border-ink-muted',
		no_show: 'border-ink-muted opacity-60'
	} as const;
	const STATUS_TEXT = {
		scheduled: 'status_scheduled',
		confirmed: 'status_confirmed',
		cancelled: 'status_cancelled',
		no_show: 'status_no_show'
	} as const;
</script>

<!-- One hour is 4rem tall; a minute is a sixtieth of it. The scroll area takes focus, so a
     keyboard can scroll it even on a day with no appointment to tab to. The hour column stays put
     while the week scrolls sideways. -->
<Card
	bind:ref={scroller}
	tabindex={0}
	role="region"
	aria-label={t('agenda_grid_label')}
	class="max-h-grid overflow-auto"
	style="--hour: 4rem"
>
	<div class="flex {onpickday === undefined ? '' : 'min-w-176'}">
		<div class="sticky left-0 z-20 w-12 shrink-0 bg-bg-100">
			{#if onpickday !== undefined}
				<div class="sticky top-0 z-20 h-14 bg-bg-100"></div>
			{/if}
			<div class="relative my-4" style="height: calc({hours.length} * var(--hour))">
				{#each hours as hour, i (hour)}
					<p
						data-hour={hour}
						class="absolute right-2 -translate-y-1/2 text-xs text-ink-muted"
						style="top: calc({i} * var(--hour))"
					>
						{String(hour).padStart(2, '0')}:00
					</p>
				{/each}
			</div>
		</div>
		<div class="flex min-w-0 flex-1">
			{#each lanes as lane (lane.day)}
				<div class="flex min-w-0 flex-1 flex-col border-l border-accent-100">
					{#if onpickday !== undefined}
						<button
							type="button"
							data-head
							class="sticky top-0 z-10 h-14 w-full bg-bg-100 text-center text-sm hover:bg-accent-100 {lane.day ===
							today
								? 'font-extrabold text-accent-700'
								: 'font-bold text-ink'}"
							onclick={() => {
								onpickday(lane.day);
							}}
						>
							<span class="block capitalize">{formatWeekdayShort(lane.day)}</span>
							<span class="block text-xs font-normal text-ink-muted">
								{formatDayShort(lane.day)}
							</span>
						</button>
					{/if}
					<div class="relative my-4" style="height: calc({hours.length} * var(--hour))">
						{#each hours as hour, i (hour)}
							<div
								class="absolute inset-x-0 border-t border-accent-100"
								style="top: calc({i} * var(--hour))"
							></div>
						{/each}
						{#each lane.placed as p (p.id)}
							{@const a = byId.get(p.id)}
							{@const box = minutes(p)}
							{#if a !== undefined}
								<div
									class="absolute p-0.5"
									style="top: calc({box.top} * var(--hour) / 60); height: calc({box.height} * var(--hour) / 60); left: {(p.col /
										p.cols) *
										100}%; width: {100 / p.cols}%"
								>
									<a
										href={resolve('/citas/[id]', { id: a.id })}
										class="block h-full min-h-tap overflow-hidden hover:bg-accent-100 rounded-sm border-l-4 bg-glass-strong py-1 {p.cols >
										1
											? 'px-1'
											: 'px-2'} {STATUS_EDGE[a.status]}"
										title="{a.client_name} · {t(STATUS_TEXT[a.status])}"
									>
										{#if box.height <= ONE_LINE_MIN}
											<p class="truncate text-sm font-bold text-ink">
												<span class="text-xs text-accent-700">{formatTime(a.starts_at, zone)}</span>
												{#if a.status === 'no_show'}
													<span class="text-xs text-ink-muted">· {t('status_no_show')}</span>
												{:else if a.status === 'confirmed'}
													<span class="sr-only">{t('status_confirmed')}</span>
												{/if}
												{a.client_name}
											</p>
										{:else}
											<p class="truncate text-xs font-bold text-accent-700">
												{formatTime(a.starts_at, zone)}
												{#if a.status === 'no_show'}
													<span class="text-ink-muted">· {t('status_no_show')}</span>
												{:else if a.status === 'confirmed'}
													<span class="sr-only">{t('status_confirmed')}</span>
												{/if}
											</p>
											<p class="truncate text-sm font-bold text-ink">{a.client_name}</p>
											<p class="truncate text-xs text-ink-muted">
												{a.services.map((s) => s.name).join(' · ')}
											</p>
										{/if}
									</a>
								</div>
							{/if}
						{/each}
					</div>
				</div>
			{/each}
		</div>
	</div>
</Card>
