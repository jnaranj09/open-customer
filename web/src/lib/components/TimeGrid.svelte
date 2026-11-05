<script lang="ts">
	import { resolve } from '$app/paths';
	import type { Appointment } from '../api/schemas';
	import { hourSpan, layoutDay, type Placed } from '../agenda';
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
	const span = $derived(hourSpan(lanes.flatMap((l) => l.placed)));
	const hours = $derived(Array.from({ length: span.to - span.from }, (_, i) => span.from + i));
	const minutes = (p: Placed): { top: number; height: number } => ({
		top: p.startMin - span.from * 60,
		height: p.endMin - p.startMin
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

<!-- One hour is 4rem tall; a minute is a sixtieth of it. -->
<div class="glass-card overflow-x-auto rounded-lg" style="--hour: 4rem">
	<div class="flex {onpickday === undefined ? '' : 'min-w-176'}">
		<div class="w-12 shrink-0">
			{#if onpickday !== undefined}
				<div class="h-14"></div>
			{/if}
			<div class="relative mt-2" style="height: calc({hours.length} * var(--hour))">
				{#each hours as hour, i (hour)}
					<p
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
							class="h-14 w-full text-center text-sm hover:bg-accent-100 {lane.day === today
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
					<div class="relative mt-2" style="height: calc({hours.length} * var(--hour))">
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
										class="block h-full overflow-hidden hover:bg-accent-100 rounded-sm border-l-4 bg-glass-strong py-1 {p.cols >
										1
											? 'px-1'
											: 'px-2'} {STATUS_EDGE[a.status]}"
										title="{a.client_name} · {t(STATUS_TEXT[a.status])}"
									>
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
									</a>
								</div>
							{/if}
						{/each}
					</div>
				</div>
			{/each}
		</div>
	</div>
</div>
