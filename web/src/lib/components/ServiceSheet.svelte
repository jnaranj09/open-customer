<script lang="ts">
	import { untrack } from 'svelte';
	import { api } from '../api/app';
	import { errorText, type ApiError } from '../api/errors';
	import type { Service } from '../api/schemas';
	import { cache } from '../cache/store.svelte';
	import { t, type MessageKey } from '../i18n';
	import { amountText, exponentOf } from '../money';
	import { servicePatch, serviceRequest, type ServiceField } from '../services';
	import { submitOnce } from '../submit';
	import ActionButton from './ActionButton.svelte';
	import BottomSheet from './ui/BottomSheet.svelte';
	import Button from './ui/Button.svelte';
	import ColorPicker from './ColorPicker.svelte';
	import SaveButton from './SaveButton.svelte';
	import TextField from './ui/TextField.svelte';

	// Add or edit one service in a sheet (design doc item 77, CUJ 2 step 1). The page mounts it when
	// she opens a service, so every opening starts from the saved values. Archiving is one tap and
	// "Restaurar servicio" undoes it: past appointments keep what they copied.
	let {
		service,
		currency,
		locale,
		accent,
		onclose,
		ondone
	}: {
		/** The service being edited; none means a new one. */
		service?: Service;
		currency: string;
		locale: string;
		/** The color a new service starts with. */
		accent: string;
		onclose: () => void;
		/** Told after a save, an archive or a restore, with the message to show. */
		ondone: (message: MessageKey) => void;
	} = $props();

	const exponent = $derived(exponentOf(currency, locale));
	let name = $state(untrack(() => service?.name ?? ''));
	let duration = $state(untrack(() => (service === undefined ? '' : String(service.duration_min))));
	let price = $state(
		untrack(() => {
			const cents = service?.price_cents ?? null;
			return cents === null ? '' : amountText(cents, exponent);
		})
	);
	let color = $state(untrack(() => service?.color ?? accent));
	let errors = $state<Partial<Record<ServiceField, MessageKey>>>({});
	let failure = $state<ApiError | null>(null);
	let busy = $state(false);
	let toggling = $state(false);

	const fieldError = (field: ServiceField, key: string): string | undefined => {
		const own = errors[field];
		return own === undefined
			? failure?.kind === 'problem'
				? failure.fields[key]
				: undefined
			: t(own);
	};
	const otherError = $derived(
		failure === null || (failure.kind === 'problem' && Object.keys(failure.fields).length > 0)
			? null
			: errorText(failure)
	);

	const save = submitOnce(
		async () => {
			failure = null;
			const request = serviceRequest({ name, duration, price, color }, exponent);
			if (!request.ok) {
				errors = request.errors;
				return;
			}
			errors = {};
			if (service === undefined) {
				const result = await api.createService(request.body);
				if (!result.ok) {
					failure = result.error;
					return;
				}
				await cache.refresh();
				ondone('service_created');
				return;
			}
			const patch = servicePatch(service, request.body);
			if (Object.keys(patch).length > 0) {
				const result = await api.updateService(service.id, patch);
				if (!result.ok) {
					failure = result.error;
					return;
				}
				await cache.refresh();
			}
			ondone('service_saved');
		},
		(pending) => {
			busy = pending;
		}
	);

	const setArchived = submitOnce(
		async (archived: boolean) => {
			if (service === undefined) {
				return;
			}
			failure = null;
			const result = await api.updateService(service.id, { archived });
			if (!result.ok) {
				failure = result.error;
				return;
			}
			await cache.refresh();
			ondone(archived ? 'service_archived' : 'service_restored');
		},
		(pending) => {
			toggling = pending;
		}
	);
</script>

<BottomSheet
	open
	title={service === undefined ? t('service_new_title') : t('service_edit_title')}
	{onclose}
>
	<form
		class="flex flex-col gap-4"
		onsubmit={(e) => {
			e.preventDefault();
			void save.run();
		}}
	>
		<TextField
			label={t('service_name')}
			bind:value={name}
			autocomplete="off"
			error={fieldError('name', 'name')}
		/>
		<div class="grid grid-cols-2 gap-3">
			<TextField
				label={t('service_duration')}
				bind:value={duration}
				inputmode="numeric"
				error={fieldError('duration', 'duration_min')}
			/>
			<TextField
				label={t('service_price', { currency })}
				bind:value={price}
				inputmode="decimal"
				error={fieldError('price', 'price_cents')}
			/>
		</div>
		<div class="flex flex-col gap-1.5">
			<span class="text-sm font-bold text-ink">{t('service_color')}</span>
			<ColorPicker bind:value={color} />
			{#if failure?.kind === 'problem' && failure.fields['color'] !== undefined}
				<p class="text-sm text-error">{failure.fields['color']}</p>
			{/if}
		</div>
		{#if otherError !== null}
			<p role="alert" class="text-sm text-error">{otherError}</p>
		{/if}
		<SaveButton {busy}>
			{t('service_save')}
		</SaveButton>
	</form>
	{#if service !== undefined}
		{#if service.archived}
			<div class="flex">
				<ActionButton
					variant="secondary"
					busy={toggling}
					onclick={() => {
						void setArchived.run(false);
					}}
				>
					{t('service_restore')}
				</ActionButton>
			</div>
		{:else}
			<p class="text-sm text-ink-muted">{t('service_archive_body')}</p>
			<div class="flex">
				<ActionButton
					variant="secondary"
					busy={toggling}
					onclick={() => {
						void setArchived.run(true);
					}}
				>
					{t('service_archive')}
				</ActionButton>
			</div>
		{/if}
	{/if}
	<Button variant="ghost" onclick={onclose}>{t('cancel')}</Button>
</BottomSheet>
