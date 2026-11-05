<script lang="ts">
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { untrack } from 'svelte';
	import { api } from '../../../lib/api/app';
	import { errorText, type ApiError } from '../../../lib/api/errors';
	import type { Appointment } from '../../../lib/api/schemas';
	import { business } from '../../../lib/business.svelte';
	import { showThenFetch } from '../../../lib/cachedLoad';
	import { attrDisplay, STATUS_NAME, statusActions } from '../../../lib/booking';
	import { cache } from '../../../lib/cache/store.svelte';
	import ActionButton from '../../../lib/components/ActionButton.svelte';
	import AppointmentForm from '../../../lib/components/AppointmentForm.svelte';
	import WriteButton from '../../../lib/components/WriteButton.svelte';
	import IconButton from '../../../lib/components/ui/IconButton.svelte';
	import Card from '../../../lib/components/ui/Card.svelte';
	import LoadFailure from '../../../lib/components/LoadFailure.svelte';
	import OverlapSheet from '../../../lib/components/OverlapSheet.svelte';
	import PhotoSection from '../../../lib/components/PhotoSection.svelte';
	import TextField from '../../../lib/components/ui/TextField.svelte';
	import Toast from '../../../lib/components/Toast.svelte';
	import { t, type MessageKey } from '../../../lib/i18n';
	import { amountText, exponentOf, formatMoney, parseAmount } from '../../../lib/money';
	import { submitOnce } from '../../../lib/submit';
	import { createToaster } from '../../../lib/toaster.svelte';
	import { formatDayLong, formatTime, todayIn } from '../../../lib/time';

	// One appointment (CUJ 3 step 5): its facts, the status buttons, the amount paid, edit mode and
	// the photos taken at it. The server holds the truth: every change waits for its answer.
	const id = $derived(page.params.id ?? '');
	const data = $derived(cache.snapshot);
	const zone = $derived(business.zone);
	const money = (minor: number): string => formatMoney(minor, business.currency, business.locale);

	let appt = $state<Appointment | null>(null);
	let loadError = $state<ApiError | null>(null);
	let editing = $state(false);
	const toaster = createToaster();

	// The cached copy shows at once (and is all there is offline); the server's answer replaces it.
	async function load(want: string): Promise<void> {
		loadError = null;
		const failure = await showThenFetch(
			untrack(() => cache.snapshot?.appointments.find((a) => a.id === want)),
			() => api.getAppointment(want),
			() => want === id,
			(value) => {
				appt = value;
			}
		);
		if (failure !== undefined) {
			loadError = failure;
		}
	}

	$effect(() => {
		const want = id;
		untrack(() => {
			editing = false;
			void load(want);
		});
	});

	const attrs = $derived.by(() => {
		const a = appt;
		if (a === null || data === null) {
			return [];
		}
		return data.attributes
			.filter((d) => d.scope === 'appointment')
			.map((d) => ({ label: d.label, text: attrDisplay(d, a.attrs[d.key]) }))
			.filter((d): d is { label: string; text: string } => d.text !== null);
	});

	// Status
	const STATUS_LABEL: Record<Appointment['status'], MessageKey> = {
		scheduled: 'appointment_reactivate',
		confirmed: 'appointment_confirm',
		cancelled: 'appointment_cancel',
		no_show: 'appointment_no_show'
	};
	const STATUS_VARIANT = {
		scheduled: 'strong',
		confirmed: 'strong',
		cancelled: 'danger',
		no_show: 'glass'
	} as const;
	let actionError = $state<ApiError | null>(null);
	let busy = $state(false);
	let overlapping = $state<{ to: Appointment['status']; items: Appointment[] } | null>(null);

	const move = submitOnce(
		async (to: Appointment['status'], allowOverlap: boolean) => {
			if (appt === null) {
				return;
			}
			actionError = null;
			const result = await api.setAppointmentStatus(appt.id, {
				status: to,
				...(allowOverlap ? { allow_overlap: true } : {})
			});
			if (result.ok) {
				overlapping = null;
				appt = result.data;
				toaster.say('appointment_updated');
				void cache.refresh();
			} else if (result.error.kind === 'overlap') {
				overlapping = { to, items: result.error.overlapping };
			} else {
				overlapping = null;
				actionError = result.error;
			}
		},
		(pending) => {
			busy = pending;
		}
	);

	// Amount paid
	const exponent = $derived(exponentOf(business.currency || 'USD', business.locale));
	let paid = $state('');
	let paidError = $state<string | null>(null);
	let paying = $state(false);
	$effect(() => {
		const a = appt;
		untrack(() => {
			paid = a?.paid_cents == null ? '' : amountText(a.paid_cents, exponent);
		});
	});

	const savePaid = submitOnce(
		async () => {
			if (appt === null) {
				return;
			}
			paidError = null;
			const minor = paid.trim() === '' ? null : parseAmount(paid, exponent);
			if (paid.trim() !== '' && minor === null) {
				paidError = t('appointment_paid_invalid');
				return;
			}
			const result = await api.updateAppointment(appt.id, { paid_cents: minor });
			if (result.ok) {
				appt = result.data;
				toaster.say('appointment_updated');
				void cache.refresh();
			} else {
				paidError = errorText(result.error);
			}
		},
		(pending) => {
			paying = pending;
		}
	);
</script>

<div class="flex flex-col gap-4">
	<header class="flex items-center gap-2">
		<IconButton
			variant="plain"
			icon="chevron-left"
			label={t('appointment_back')}
			href={resolve('/')}
		/>
		<h1 class="text-3xl font-extrabold text-ink">
			{editing ? t('appointment_edit_title') : t('appointment_title')}
		</h1>
	</header>

	{#if loadError !== null}
		<LoadFailure
			error={loadError}
			onretry={() => {
				void load(id);
			}}
		/>
	{:else if appt === null || data === null}
		<p role="status" class="px-2 text-base text-ink-muted">{t('appointment_loading')}</p>
	{:else if editing}
		<AppointmentForm
			{data}
			appointment={appt}
			onsaved={(saved: Appointment) => {
				appt = saved;
				editing = false;
				toaster.say('appointment_updated');
			}}
			oncancel={() => {
				editing = false;
			}}
		/>
	{:else}
		<div class="flex flex-col gap-4 lg:max-w-190">
			<Card as="section" radius="xl" pad="section" class="flex flex-col gap-4">
				<div class="flex items-start gap-3">
					<div class="min-w-0 flex-1">
						<h2 class="text-xl font-bold text-ink">{appt.client_name}</h2>
						<p class="text-base text-ink">
							{formatDayLong(todayIn(zone, new Date(appt.starts_at)))}
						</p>
						<p class="text-base text-ink">
							{formatTime(appt.starts_at, zone)} – {formatTime(appt.ends_at, zone)}
						</p>
					</div>
					<p
						class="rounded-pill bg-glass-tint px-3 py-1 text-sm font-bold text-accent-700"
						aria-label="{t('appointment_status_label')}: {t(STATUS_NAME[appt.status])}"
					>
						{t(STATUS_NAME[appt.status])}
					</p>
				</div>

				<ul class="flex flex-col gap-1">
					{#each appt.services as line, i (i)}
						<li class="flex justify-between gap-3 text-base text-ink">
							<span>{line.name}</span>
							{#if line.price_cents !== null}
								<span>{money(line.price_cents)}</span>
							{/if}
						</li>
					{:else}
						<li class="text-base text-ink-muted">{t('appointment_no_services')}</li>
					{/each}
				</ul>
				<p class="text-base font-bold text-ink">
					{t('appointment_total', { amount: money(appt.total_cents) })}
				</p>

				{#if attrs.length > 0 || appt.notes !== null}
					<dl class="flex flex-col gap-1 text-base">
						{#each attrs as a (a.label)}
							<div class="flex gap-2">
								<dt class="font-bold text-ink">{a.label}:</dt>
								<dd class="text-ink">{a.text}</dd>
							</div>
						{/each}
						{#if appt.notes !== null}
							<div class="flex gap-2">
								<dt class="font-bold text-ink">{t('appointment_notes')}:</dt>
								<dd class="text-ink">{appt.notes}</dd>
							</div>
						{/if}
					</dl>
				{/if}

				<div class="flex flex-wrap items-start gap-3">
					{#each statusActions(appt.status) as to (to)}
						<ActionButton
							variant={STATUS_VARIANT[to]}
							{busy}
							onclick={() => {
								void move.run(to, false);
							}}
						>
							{t(STATUS_LABEL[to])}
						</ActionButton>
					{/each}
					<ActionButton
						variant="ghost"
						onclick={() => {
							editing = true;
						}}
					>
						{t('appointment_edit')}
					</ActionButton>
				</div>
				{#if actionError !== null}
					<p role="alert" class="text-sm text-error">{errorText(actionError)}</p>
				{/if}
			</Card>

			<Card as="section" radius="xl" pad="section" class="flex flex-col gap-3">
				<form
					class="flex items-start gap-3"
					onsubmit={(e) => {
						e.preventDefault();
						void savePaid.run();
					}}
				>
					<div class="min-w-0 flex-1">
						<TextField
							label={t('appointment_paid')}
							inputmode="decimal"
							bind:value={paid}
							error={paidError ?? undefined}
						/>
					</div>
					<div class="mt-7 w-40 shrink-0">
						<WriteButton variant="glass" type="submit" busy={paying}>
							{t('appointment_paid_save')}
						</WriteButton>
					</div>
				</form>
			</Card>
		</div>
	{/if}

	<!-- Outside the view and edit branches, so editing does not read the photos again or lose an
	     upload error; hidden while the form shows. -->
	{#if appt !== null && loadError === null}
		<div class="lg:max-w-190 {editing ? 'hidden' : ''}">
			{#key appt.id}
				<PhotoSection clientId={appt.client_id} appointmentId={appt.id} empty="photos_empty" />
			{/key}
		</div>
	{/if}
</div>

{#if toaster.current !== null}
	{#key toaster.current.n}
		<Toast message={toaster.current.message} />
	{/key}
{/if}

<OverlapSheet
	open={overlapping !== null}
	overlapping={overlapping?.items ?? []}
	{zone}
	{busy}
	onconfirm={() => {
		if (overlapping !== null) {
			void move.run(overlapping.to, true);
		}
	}}
	onclose={() => {
		overlapping = null;
	}}
/>
