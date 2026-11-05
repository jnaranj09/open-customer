<script lang="ts">
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { untrack } from 'svelte';
	import { api } from '../../../lib/api/app';
	import { errorText, type ApiError } from '../../../lib/api/errors';
	import type { Appointment, Media } from '../../../lib/api/schemas';
	import { attrDisplay, statusActions } from '../../../lib/booking';
	import { cache } from '../../../lib/cache/store.svelte';
	import AppointmentForm from '../../../lib/components/AppointmentForm.svelte';
	import Button from '../../../lib/components/Button.svelte';
	import Icon from '../../../lib/components/icons/Icon.svelte';
	import OverlapSheet from '../../../lib/components/OverlapSheet.svelte';
	import PhotoGrid from '../../../lib/components/PhotoGrid.svelte';
	import TextField from '../../../lib/components/TextField.svelte';
	import Toast from '../../../lib/components/Toast.svelte';
	import { t, type MessageKey } from '../../../lib/i18n';
	import { amountText, exponentOf, formatMoney, parseAmount } from '../../../lib/money';
	import { toJpegs } from '../../../lib/photos';
	import { submitOnce } from '../../../lib/submit';
	import { formatDayLong, formatTime, todayIn } from '../../../lib/time';

	// One appointment (CUJ 3 step 5): its facts, the status buttons, the amount paid, edit mode and
	// the photos taken at it. The server holds the truth: every change waits for its answer.
	const id = $derived(page.params.id ?? '');
	const data = $derived(cache.snapshot);
	const zone = $derived(
		data?.settings.timezone ?? Intl.DateTimeFormat().resolvedOptions().timeZone
	);
	const currency = $derived(data?.settings.currency ?? '');
	const locale = $derived(data?.settings.locale ?? 'es');
	const money = (minor: number): string => formatMoney(minor, currency, locale);

	let appt = $state<Appointment | null>(null);
	let loadError = $state<ApiError | null>(null);
	let editing = $state(false);
	let toast = $state<{ n: number; message: string } | null>(null);
	let toasts = 0;

	function say(message: MessageKey): void {
		toasts += 1;
		toast = { n: toasts, message: t(message) };
	}

	// The cached copy shows at once (and is all there is offline); the server's answer replaces it.
	async function load(want: string): Promise<void> {
		loadError = null;
		const cached = untrack(() => cache.snapshot?.appointments.find((a) => a.id === want));
		appt = cached ?? null;
		const result = await api.getAppointment(want);
		if (want !== id) {
			return;
		}
		if (result.ok) {
			appt = result.data;
		} else if (result.error.kind !== 'offline' || cached === undefined) {
			loadError = result.error;
		}
	}

	$effect(() => {
		const want = id;
		untrack(() => {
			editing = false;
			photos = null;
			listError = null;
			photosError = null;
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

	const offlineOff = $derived(
		cache.offline ? { disabled: true as const, disabledReason: t('offline_cannot_save') } : {}
	);

	// Status
	const STATUS_LABEL: Record<Appointment['status'], MessageKey> = {
		scheduled: 'appointment_reactivate',
		confirmed: 'appointment_confirm',
		cancelled: 'appointment_cancel',
		no_show: 'appointment_no_show'
	};
	const STATUS_VARIANT = {
		scheduled: 'primary',
		confirmed: 'primary',
		cancelled: 'danger',
		no_show: 'secondary'
	} as const;
	const STATUS_NAME: Record<Appointment['status'], MessageKey> = {
		scheduled: 'status_scheduled',
		confirmed: 'status_confirmed',
		cancelled: 'status_cancelled',
		no_show: 'status_no_show'
	};

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
				say('appointment_updated');
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
	const exponent = $derived(exponentOf(currency || 'USD', locale));
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
				say('appointment_updated');
				void cache.refresh();
			} else {
				paidError = errorText(result.error);
			}
		},
		(pending) => {
			paying = pending;
		}
	);

	// Photos
	let photos = $state<Media[] | null>(null);
	// The list's own failure (retried when the appointment changes) and the last upload's, apart:
	// reloading the list must not wipe an upload error.
	let listError = $state<string | null>(null);
	let photosError = $state<string | null>(null);
	let uploading = $state(false);

	async function loadPhotos(clientId: string, apptId: string): Promise<void> {
		const result = await api.listClientMedia(clientId);
		if (result.ok) {
			photos = result.data.filter((m) => m.appointment_id === apptId);
			listError = null;
		} else {
			listError = result.error.kind === 'offline' ? t('photos_offline') : errorText(result.error);
		}
	}

	$effect(() => {
		const a = appt;
		if (a !== null && photos === null) {
			untrack(() => {
				void loadPhotos(a.client_id, a.id);
			});
		}
	});

	const upload = submitOnce(
		async (file: File) => {
			if (appt === null) {
				return;
			}
			photosError = null;
			let jpegs;
			try {
				jpegs = await toJpegs(file);
			} catch {
				console.warn('code=photo_decode_failed');
				photosError = t('photo_unreadable');
				return;
			}
			const result = await api.uploadMedia({ ...jpegs, appointment_id: appt.id });
			if (!result.ok) {
				photosError = errorText(result.error);
				return;
			}
			await loadPhotos(appt.client_id, appt.id);
			say('photo_added');
		},
		(pending) => {
			uploading = pending;
		}
	);

	function picked(e: Event & { currentTarget: HTMLInputElement }): void {
		const file = e.currentTarget.files?.[0];
		e.currentTarget.value = '';
		if (file !== undefined) {
			void upload.run(file);
		}
	}

	async function archive(photo: Media): Promise<string | null> {
		const result = await api.archiveMedia(photo.id);
		if (!result.ok) {
			return errorText(result.error);
		}
		photos = (photos ?? []).filter((m) => m.id !== photo.id);
		say('photo_archived');
		return null;
	}
</script>

<div class="flex flex-col gap-4">
	<header class="flex items-center gap-2">
		<a
			href={resolve('/')}
			title={t('appointment_back')}
			aria-label={t('appointment_back')}
			class="grid size-11 place-items-center rounded-pill text-ink hover:bg-accent-100"
		>
			<Icon name="chevron-left" class="size-5.5" />
		</a>
		<h1 class="font-display text-3xl font-semibold text-ink">
			{editing ? t('appointment_edit_title') : t('appointment_title')}
		</h1>
	</header>

	{#if loadError !== null}
		<div role="alert" class="glass-card flex flex-col gap-3 rounded-lg px-5 py-3">
			<p class="text-base text-error">{errorText(loadError)}</p>
			<Button
				variant="ghost"
				onclick={() => {
					void load(id);
				}}
			>
				{t('agenda_retry')}
			</Button>
		</div>
	{:else if appt === null || data === null}
		<p role="status" class="px-2 text-base text-ink-muted">{t('appointment_loading')}</p>
	{:else if editing}
		<AppointmentForm
			{data}
			appointment={appt}
			onsaved={(saved: Appointment) => {
				appt = saved;
				editing = false;
				say('appointment_updated');
			}}
			oncancel={() => {
				editing = false;
			}}
		/>
	{:else}
		<div class="flex flex-col gap-4 lg:max-w-190">
			<section class="glass-card flex flex-col gap-4 rounded-xl p-5 lg:p-8">
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
						<div class="min-w-36 flex-1">
							<Button
								variant={STATUS_VARIANT[to]}
								{busy}
								{...offlineOff}
								onclick={() => {
									void move.run(to, false);
								}}
							>
								{t(STATUS_LABEL[to])}
							</Button>
						</div>
					{/each}
					<div class="min-w-36 flex-1">
						<Button
							variant="ghost"
							{...offlineOff}
							onclick={() => {
								editing = true;
							}}
						>
							{t('appointment_edit')}
						</Button>
					</div>
				</div>
				{#if actionError !== null}
					<p role="alert" class="text-sm text-error">{errorText(actionError)}</p>
				{/if}
			</section>

			<section class="glass-card flex flex-col gap-3 rounded-xl p-5 lg:p-8">
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
						<Button variant="secondary" type="submit" busy={paying} {...offlineOff}>
							{t('appointment_paid_save')}
						</Button>
					</div>
				</form>
			</section>

			<section class="glass-card flex flex-col gap-3 rounded-xl p-5 lg:p-8">
				<div class="flex items-center gap-3">
					<h2 class="flex-1 text-xl font-bold text-ink">{t('photos_title')}</h2>
					<label
						class="relative inline-flex min-h-tap items-center justify-center gap-2 rounded-pill border border-accent-300 bg-accent-100 px-6 py-3 text-sm font-bold text-accent-700 focus-within:outline-2 hover:bg-accent-300 {cache.offline ||
						uploading
							? 'pointer-events-none opacity-60'
							: ''}"
					>
						<Icon name="plus" />
						{uploading ? t('photo_adding') : t('photo_add')}
						<input
							type="file"
							accept="image/*"
							disabled={cache.offline || uploading}
							onchange={picked}
							class="absolute inset-0 size-full cursor-pointer opacity-0"
						/>
					</label>
				</div>
				{#if photosError !== null || listError !== null}
					<p role="alert" class="text-sm text-error">{photosError ?? listError}</p>
				{:else if photos !== null && photos.length === 0}
					<p class="text-sm text-ink-muted">{t('photos_empty')}</p>
				{/if}
				{#if photos !== null && photos.length > 0}
					<PhotoGrid {photos} onarchive={archive} />
				{/if}
			</section>
		</div>
	{/if}
</div>

{#if toast !== null}
	{#key toast.n}
		<Toast message={toast.message} />
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
