<script lang="ts">
	import type { Appointment } from '../api/schemas';
	import { t } from '../i18n';
	import { addDays, weekStart } from '../time';
	import TimeGrid from './TimeGrid.svelte';

	let {
		day,
		appointments,
		zone,
		today,
		onpickday
	}: {
		/** Any day of the week to show; the week runs Monday to Sunday. */
		day: string;
		appointments: Appointment[];
		zone: string;
		today: string;
		onpickday: (day: string) => void;
	} = $props();

	const days = $derived(Array.from({ length: 7 }, (_, i) => addDays(weekStart(day), i)));
	// Like the day view, an empty week says so above its grid (cancelled ones are not drawn).
	const empty = $derived(!appointments.some((a) => a.status !== 'cancelled'));
</script>

{#if empty}
	<p class="px-2 text-base text-ink-muted">{t('agenda_empty_week')}</p>
{/if}
<TimeGrid {days} {appointments} {zone} {today} {onpickday} />
