<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import type { Appointment } from '../api/schemas';
	import { blankForm, validDay, type Line } from '../booking';
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
	import TextField from './ui/TextField.svelte';

	// "Agendar próxima cita" (next-appointment design, section 4): one date choice per come-back
	// rule that applies to `appointment`, each with the services it books, and a field for another
	// date. The picked day becomes the Nueva cita draft (same client, same start time; notes and
	// attributes start empty), and Nueva cita opens in this page's place in the history, so its Save
	// and its back arrow return here without a loop. Nothing is created here.
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

	const zone = $derived(data.settings.timezone);
	const plan = $derived(
		nextVisit(appointment, data.rules, data.templates, data.services, zone, clock.today(zone))
	);

	let otherDay = $state('');
	// At most one message: the replace question, or the failed write.
	let message = $state<'none' | 'confirm_replace' | 'failed'>('none');
	// What she tapped last, so "Reemplazar" books that and nothing else.
	let asked = $state<{ day: string; lines: Line[] } | null>(null);
	let busy = $state(false);

	// Inside `submitOnce`, so a second tap while the draft is written does nothing.
	const pick = submitOnce(
		async (day: string, lines: Line[], replace: boolean): Promise<void> => {
			message = 'none';
			asked = { day, lines };
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

	function close(): void {
		message = 'none';
		otherDay = '';
		onclose();
	}
</script>

<BottomSheet {open} title={t('appointment_next')} onclose={close}>
	{#if plan.choices.length > 0}
		<ul class="flex flex-col gap-3">
			{#each plan.choices as choice (choice.days)}
				<li>
					<Button
						variant="glass"
						{busy}
						onclick={() => {
							void pick.run(choice.day, choice.lines, false);
						}}
					>
						<span class="flex min-w-0 flex-col break-words">
							<span>{choiceTitle(choice)}</span>
							<span class="font-normal">{linesText(choice.lines, data.services)}</span>
						</span>
					</Button>
				</li>
			{/each}
		</ul>
	{/if}

	<div class="flex flex-col gap-3">
		<TextField
			label={t('next_other_date')}
			type="date"
			bind:value={otherDay}
			hint={linesText(plan.otherDate, data.services)}
		/>
		<Button
			variant="strong"
			{busy}
			onclick={() => {
				void pick.run(otherDay, plan.otherDate, false);
			}}
			{...validDay(otherDay)
				? {}
				: { disabled: true as const, disabledReason: t('appointment_day_needed') }}
		>
			{t('next_use_date')}
		</Button>
	</div>

	{#if message === 'confirm_replace'}
		<ReplaceDraftPrompt
			{busy}
			onreplace={() => {
				if (asked !== null) {
					void pick.run(asked.day, asked.lines, true);
				}
			}}
			oncancel={() => {
				message = 'none';
			}}
		/>
	{:else}
		{#if message === 'failed'}
			<Callout role="alert" class="text-sm">{t('cita_handoff_failed')}</Callout>
		{/if}
		<!-- Not rendered while the question shows: one "Cancelar" on screen, the question's. -->
		<Button variant="glass" onclick={close}>{t('cancel')}</Button>
	{/if}
</BottomSheet>
