<script lang="ts">
	import { untrack } from 'svelte';
	import { api } from '../api/app';
	import { errorText, type ApiError } from '../api/errors';
	import type { Appointment, Attribute } from '../api/schemas';
	import {
		createBody,
		endShown,
		formErrors,
		formFrom,
		FormSchema,
		patchBody,
		restoreForm,
		totalOf,
		withSavedLines,
		type AttrValue,
		type Form
	} from '../booking';
	import { draftKey } from '../cache/drafts';
	import { formDraft } from '../cache/formDraft.svelte';
	import type { CacheSnapshot } from '../cache/db';
	import { cache } from '../cache/store.svelte';
	import { t } from '../i18n';
	import { formatMoney } from '../money';
	import { submitOnce } from '../submit';
	import AttributeField from './AttributeField.svelte';
	import FormCard from './FormCard.svelte';
	import ClientPicker from './ClientPicker.svelte';
	import OverlapSheet from './OverlapSheet.svelte';
	import ServicePicker from './ServicePicker.svelte';
	import TextField from './TextField.svelte';

	// The booking form, shared by "Nueva cita" and the detail's edit mode (design doc item 75).
	// Creating, it keeps a draft (item 67) and asks for the client; editing, the client is fixed and
	// only what changed is sent.
	let {
		data,
		appointment,
		day,
		onsaved,
		oncancel
	}: {
		data: CacheSnapshot;
		/** The appointment being edited; none means a new one. */
		appointment?: Appointment;
		/** The day a new appointment starts on. */
		day?: string;
		onsaved: (saved: Appointment) => void;
		oncancel?: () => void;
	} = $props();

	const creating = untrack(() => appointment === undefined);
	const zone = $derived(data.settings.timezone);
	const defs = $derived(data.attributes.filter((a) => a.scope === 'appointment' && !a.archived));
	const quote = $derived(withSavedLines(data.services, appointment?.services ?? []));

	const initial: Form = untrack(() =>
		appointment === undefined
			? { clientId: null, lines: [], day: day ?? '', time: '', end: null, notes: '', attrs: {} }
			: formFrom(appointment, data.settings.timezone, defs)
	);
	let form = $state<Form>(structuredClone(initial));

	// A draft of a new booking survives a reload or a closed app until it is saved.
	const draft = formDraft(
		creating ? draftKey('cita', 'new') : null,
		FormSchema,
		initial,
		() => $state.snapshot(form),
		(saved) => {
			form = restoreForm(saved, {
				clients: new Set(data.clients.map((c) => c.id)),
				services: new Set(data.services.map((s) => s.id)),
				attrKeys: new Set(defs.map((a) => a.key))
			});
		}
	);

	const end = $derived(endShown(form, appointment ?? null, zone, quote));
	const total = $derived(
		formatMoney(totalOf(form.lines, quote), data.settings.currency, data.settings.locale)
	);

	let attempted = $state(false);
	const hadServices = untrack(() => (appointment?.services.length ?? 0) > 0);
	const errors = $derived(attempted ? formErrors(form, creating, hadServices) : {});
	let failure = $state<ApiError | null>(null);
	let overlapping = $state<Appointment[] | null>(null);
	let busy = $state(false);

	const fieldError = (name: string): string | undefined =>
		failure?.kind === 'problem' ? failure.fields[name] : undefined;

	const attrValue = (attr: Attribute): AttrValue =>
		form.attrs[attr.key] ?? (attr.type === 'multiselect' ? [] : '');
	const setterFor =
		(key: string) =>
		(v: AttrValue): void => {
			form.attrs[key] = v;
		};
	const setEnd = (v: string): void => {
		form.end = v === '' ? null : v;
	};

	const submit = submitOnce(
		async (allowOverlap: boolean) => {
			failure = null;
			attempted = true;
			if (Object.keys(formErrors(form, creating, hadServices)).length > 0) {
				return;
			}
			const extra = allowOverlap ? { allow_overlap: true } : {};
			let result;
			if (appointment === undefined) {
				result = await api.createAppointment({ ...createBody(form, zone, defs), ...extra });
			} else {
				const patch = patchBody(appointment, form, zone, defs);
				if (Object.keys(patch).length === 0) {
					onsaved(appointment);
					return;
				}
				result = await api.updateAppointment(appointment.id, { ...patch, ...extra });
			}
			if (result.ok) {
				overlapping = null;
				await draft.clear();
				await cache.refresh();
				onsaved(result.data);
			} else if (result.error.kind === 'overlap') {
				overlapping = result.error.overlapping;
			} else {
				overlapping = null;
				failure = result.error;
			}
		},
		(pending) => {
			busy = pending;
		}
	);
</script>

<FormCard
	restored={draft.restored}
	label={creating ? t('appointment_save') : t('appointment_save_changes')}
	{busy}
	onsubmit={() => {
		void submit.run(false);
	}}
	onstartover={() => {
		void draft.startOver();
	}}
	{oncancel}
>
	{#if appointment === undefined}
		<ClientPicker
			clients={data.clients}
			bind:value={form.clientId}
			error={errors.client === undefined ? fieldError('client_id') : t(errors.client)}
			oncreated={() => {
				void cache.refresh();
			}}
		/>
	{/if}

	<div class="grid gap-4 lg:grid-cols-2 lg:gap-8">
		<div class="flex flex-col gap-4">
			<ServicePicker services={quote} bind:lines={form.lines} />
			{#if fieldError('service_ids') !== undefined}
				<p role="alert" class="text-sm text-error">{fieldError('service_ids')}</p>
			{/if}
			<div class="grid grid-cols-2 gap-3">
				<TextField
					label={t('appointment_day')}
					type="date"
					bind:value={form.day}
					error={errors.day === undefined ? fieldError('starts_at') : t(errors.day)}
				/>
				<TextField
					label={t('appointment_time')}
					type="time"
					bind:value={form.time}
					error={errors.time === undefined ? undefined : t(errors.time)}
				/>
			</div>
			<div class="grid grid-cols-2 items-end gap-3">
				<TextField
					label={t('appointment_end')}
					type="time"
					bind:value={() => end, setEnd}
					error={errors.end === undefined ? fieldError('ends_at') : t(errors.end)}
				/>
				<p class="pb-3 text-base font-bold text-ink">{t('appointment_total', { amount: total })}</p>
			</div>
			{#if form.end !== null && form.lines.length > 0}
				<button
					type="button"
					class="min-h-tap self-start rounded-pill px-4 text-sm font-bold text-accent-700 hover:bg-accent-100"
					onclick={() => {
						form.end = null;
					}}
				>
					{t('appointment_end_auto')}
				</button>
			{/if}
		</div>

		<div class="flex flex-col gap-4">
			{#each defs as attr (attr.id)}
				<AttributeField
					{attr}
					bind:value={() => attrValue(attr), setterFor(attr.key)}
					error={fieldError(`attrs.${attr.key}`)}
				/>
			{/each}
			<TextField
				label={t('appointment_notes')}
				bind:value={form.notes}
				error={fieldError('notes')}
			/>
		</div>
	</div>

	{#if failure !== null}
		<p role="alert" class="text-sm text-error">{errorText(failure)}</p>
	{/if}
</FormCard>

<OverlapSheet
	open={overlapping !== null}
	overlapping={overlapping ?? []}
	{zone}
	{busy}
	onconfirm={() => {
		void submit.run(true);
	}}
	onclose={() => {
		overlapping = null;
	}}
/>
