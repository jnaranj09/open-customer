<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import type { Path } from '$app/types';
	import { dayParam, returnTo } from '../../../lib/booking';
	import { cache } from '../../../lib/cache/store.svelte';
	import { clock } from '../../../lib/clock.svelte';
	import AppointmentForm from '../../../lib/components/AppointmentForm.svelte';
	import Icon from '../../../lib/components/icons/Icon.svelte';
	import { t } from '../../../lib/i18n';

	// "Nueva cita" (CUJ 3). `dia` is the day the link was opened from; `volver` is where "save"
	// goes back to (Hoy by default).
	const zone = $derived(
		cache.snapshot?.settings.timezone ?? Intl.DateTimeFormat().resolvedOptions().timeZone
	);
	const day = $derived(dayParam(page.url.searchParams.get('dia'), clock.today(zone)));
	// `returnTo` checked the path is inside the app; `resolve` types a path without its first slash.
	const back = $derived(resolve(returnTo(page.url.searchParams.get('volver')).slice(1) as Path));
</script>

<div class="flex flex-col gap-4">
	<header class="flex items-center gap-2">
		<a
			href={back}
			title={t('appointment_back')}
			aria-label={t('appointment_back')}
			class="grid size-11 place-items-center rounded-pill text-ink hover:bg-accent-100"
		>
			<Icon name="chevron-left" class="size-5.5" />
		</a>
		<h1 class="font-display text-3xl font-semibold text-ink">{t('appointment_new')}</h1>
	</header>

	{#if cache.snapshot === null}
		<p role="status" class="px-2 text-base text-ink-muted">{t('agenda_loading')}</p>
	{:else}
		<AppointmentForm
			data={cache.snapshot}
			{day}
			onsaved={() => {
				void goto(back);
			}}
			oncancel={() => {
				void goto(back);
			}}
		/>
	{/if}
</div>
