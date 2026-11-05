<script lang="ts">
	import type { Appointment } from '../api/schemas';
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
</script>

<TimeGrid {days} {appointments} {zone} {today} {onpickday} />
