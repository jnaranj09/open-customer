<script lang="ts">
	import type { Appointment } from '../api/schemas';
	import { t } from '../i18n';
	import TimeGrid from './TimeGrid.svelte';

	let {
		day,
		appointments,
		zone,
		today
	}: { day: string; appointments: Appointment[]; zone: string; today: string } = $props();

	const empty = $derived(!appointments.some((a) => a.status !== 'cancelled'));
</script>

{#if empty}
	<p class="px-2 text-base text-ink-muted">{t('agenda_empty_day')}</p>
{/if}
<TimeGrid days={[day]} {appointments} {zone} {today} />
