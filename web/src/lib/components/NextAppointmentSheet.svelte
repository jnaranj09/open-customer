<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import type { Appointment } from '../api/schemas';
	import { blankForm, validDay } from '../booking';
	import { handOff } from '../cache/citaHandoff';
	import type { CacheSnapshot } from '../cache/db';
	import { clock } from '../clock.svelte';
	import { t } from '../i18n';
	import { choiceTitle, linesText, nextVisit } from '../nextVisit';
	import { submitOnce } from '../submit';
	import { formatTime } from '../time';
	import ReplaceDraftPrompt from './ReplaceDraftPrompt.svelte';
	import BottomSheet from './ui/BottomSheet.svelte';
	import Button from './ui/Button.svelte';
	import Callout from './ui/Callout.svelte';
	import SelectField from './ui/SelectField.svelte';
	import TextField from './ui/TextField.svelte';

	// "Agendar próxima cita" (next-appointment design, section 4): a dropdown with one date per
	// come-back rule that applies to `appointment`, plus "Otra fecha" for a day she types, and under
	// it the services the selected date books. "Usar esta fecha" makes that the Nueva cita draft
	// (same client, same start time; notes and attributes start empty), and Nueva cita opens in
	// this page's place in the history, so its Save and its back arrow return here without a loop.
	// Nothing is created here.
	let {
		open,
		appointment,
		data,
		onclose
	}: {
		open: boolean;
		appointment: Appointment;
		data: CacheSnapshot;
		onclose: () => void;
	} = $props();

	/** The dropdown value of "Otra fecha"; a date choice has its number of days. */
	const OTHER = 'other';

	const zone = $derived(data.settings.timezone);
	const plan = $derived(
		nextVisit(appointment, data.rules, data.templates, data.services, zone, clock.today(zone))
	);
	const options = $derived([
		...plan.choices.map((choice) => ({ id: String(choice.days), label: choiceTitle(choice) })),
		{ id: OTHER, label: t('next_other_date') }
	]);

	// What she picked in the dropdown; until she does, the first choice (the fewest days), or
	// "Otra fecha" when no rule gives a date.
	let picked = $state<string | null>(null);
	const selected = $derived(picked ?? options[0]?.id ?? OTHER);
	const choice = $derived(plan.choices.find((c) => String(c.days) === selected));
	let otherDay = $state('');
	// The day and the services "Usar esta fecha" books now.
	const day = $derived(choice?.day ?? otherDay);
	const lines = $derived(choice?.lines ?? plan.otherDate);

	// At most one message: the replace question, or the failed write. It goes with her next pick.
	let message = $state<'none' | 'confirm_replace' | 'failed'>('none');
	let busy = $state(false);

	function select(id: string): void {
		message = 'none';
		picked = id;
	}

	// Inside `submitOnce`, so a second tap while the draft is written does nothing. It books what
	// is selected when it runs, so "Reemplazar" books the date on screen.
	const use = submitOnce(
		async (replace: boolean): Promise<void> => {
			message = 'none';
			const result = await handOff(
				{
					...blankForm(day),
					clientId: appointment.client_id,
					lines,
					time: formatTime(appointment.starts_at, zone)
				},
				replace
			);
			if (result === 'needs_confirm') {
				message = 'confirm_replace';
			} else if (result === 'failed') {
				message = 'failed';
			} else {
				await goto(resolve(`/citas/nueva?volver=/citas/${appointment.id}` as const), {
					replaceState: true
				});
			}
		},
		(pending) => {
			busy = pending;
		}
	);

	// The message opens under the button; on a short phone that can be below the fold, so it is
	// brought into view when it appears.
	const intoView = (slot: HTMLElement): void => {
		slot.scrollIntoView({ block: 'nearest' });
	};

	function close(): void {
		message = 'none';
		picked = null;
		otherDay = '';
		onclose();
	}
</script>

<BottomSheet {open} title={t('appointment_next')} onclose={close}>
	{#if plan.choices.length > 0}
		<SelectField label={t('appointment_day')} bind:value={() => selected, select} {options} />
	{/if}
	{#if choice === undefined}
		<TextField label={t('next_other_date')} type="date" bind:value={otherDay} />
	{/if}
	<p class="text-sm text-ink">
		<span class="font-bold">{t('appointment_services')}:</span>
		{linesText(lines, data.services)}
	</p>
	<Button
		variant="strong"
		{busy}
		onclick={() => {
			void use.run(false);
		}}
		{...validDay(day)
			? {}
			: { disabled: true as const, disabledReason: t('appointment_day_needed') }}
	>
		{t('next_use_date')}
	</Button>

	{#if message !== 'none'}
		<div {@attach intoView}>
			{#if message === 'confirm_replace'}
				<ReplaceDraftPrompt
					{busy}
					onreplace={() => {
						void use.run(true);
					}}
					oncancel={() => {
						message = 'none';
					}}
				/>
			{:else}
				<Callout role="alert" class="text-sm">{t('cita_handoff_failed')}</Callout>
			{/if}
		</div>
	{/if}
	<!-- Not rendered while the question shows: one "Cancelar" on screen, the question's. -->
	{#if message !== 'confirm_replace'}
		<Button variant="glass" onclick={close}>{t('cancel')}</Button>
	{/if}
</BottomSheet>
