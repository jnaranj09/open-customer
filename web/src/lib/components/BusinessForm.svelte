<script lang="ts">
	import Card from './ui/Card.svelte';
	import { untrack } from 'svelte';
	import { api } from '../api/app';
	import { errorText, type ApiError } from '../api/errors';
	import type { Settings } from '../api/schemas';
	import { cache } from '../cache/store.svelte';
	import { applyAccent } from '../color';
	import { iconVersion } from '../iconVersion';
	import { t } from '../i18n';
	import { toIconPngs } from '../photos';
	import { timeZoneOptions } from '../setup';
	import { businessPatch, LAST_PUSH_HOUR } from '../settingsPage';
	import { submitOnce } from '../submit';
	import { toaster } from '../toaster.svelte';
	import Button from './ui/Button.svelte';
	import ColorPicker from './ColorPicker.svelte';
	import FormCard from './FormCard.svelte';
	import IconFileInput from './IconFileInput.svelte';
	import SelectField from './ui/SelectField.svelte';
	import TextField from './ui/TextField.svelte';

	// The Negocio form: name, time zone, color and the hour of the daily notice, saved together.
	// The icon is its own card, because it is a separate upload. Currency, country and language are
	// fixed at setup (the API refuses them), so they are not here.
	let { settings }: { settings: Settings } = $props();

	let name = $state(untrack(() => settings.name));
	let timezone = $state(untrack(() => settings.timezone));
	let color = $state(untrack(() => settings.accent_color));
	let pushHour = $state(untrack(() => String(settings.push_hour)));
	let failure = $state<ApiError | null>(null);
	let attempted = $state(false);
	let busy = $state(false);

	const zones = timeZoneOptions(
		Intl,
		untrack(() => settings.timezone)
	);
	const hours = Array.from({ length: LAST_PUSH_HOUR + 1 }, (_, h) => ({
		id: String(h),
		label: `${String(h).padStart(2, '0')}:00`
	}));

	// Shows the color as she picks it, and puts the saved one back when she leaves without saving.
	$effect(() => {
		applyAccent(document.documentElement, color);
		return () => {
			applyAccent(document.documentElement, cache.snapshot?.settings.accent_color ?? null);
		};
	});

	const fieldError = (field: string): string | undefined =>
		failure?.kind === 'problem' ? failure.fields[field] : undefined;
	const nameError = $derived(
		attempted && name.trim() === '' ? t('field_required') : fieldError('name')
	);
	const otherError = $derived(
		failure === null || ['name', 'timezone', 'accent_color', 'push_hour'].some(fieldError)
			? null
			: errorText(failure)
	);

	const save = submitOnce(
		async () => {
			failure = null;
			attempted = true;
			if (name.trim() === '') {
				return;
			}
			const patch = businessPatch(settings, { name, timezone, color, pushHour });
			if (Object.keys(patch).length > 0) {
				const result = await api.updateSettings(patch);
				if (!result.ok) {
					failure = result.error;
					return;
				}
				await cache.refresh();
			}
			toaster.say('business_saved');
		},
		(pending) => {
			busy = pending;
		}
	);

	let iconFile = $state<File | null>(null);
	let iconError = $state<string | null>(null);
	let iconBusy = $state(false);

	const uploadIcon = submitOnce(
		async () => {
			if (iconFile === null) {
				return;
			}
			iconError = null;
			let icons;
			try {
				icons = await toIconPngs(iconFile);
			} catch {
				console.warn('code=icon_decode_failed');
				iconError = t('setup_icon_unreadable');
				return;
			}
			const result = await api.uploadIcon(icons);
			if (!result.ok) {
				iconError = errorText(result.error);
				return;
			}
			iconFile = null;
			iconVersion.update((v) => v + 1);
			await cache.refresh();
			toaster.say('business_icon_saved');
		},
		(pending) => {
			iconBusy = pending;
		}
	);
</script>

<FormCard
	label={t('business_save')}
	{busy}
	onsubmit={() => {
		void save.run();
	}}
>
	<TextField
		label={t('setup_business_label')}
		bind:value={name}
		autocomplete="organization"
		error={nameError}
	/>
	{#if zones === null}
		<TextField
			label={t('setup_timezone_label')}
			bind:value={timezone}
			hint={t('setup_timezone_hint')}
			error={fieldError('timezone')}
		/>
	{:else}
		<SelectField
			label={t('setup_timezone_label')}
			bind:value={timezone}
			options={zones}
			error={fieldError('timezone')}
		/>
	{/if}
	<div class="flex flex-col gap-1.5">
		<span class="text-sm font-bold text-ink">{t('business_color')}</span>
		<ColorPicker bind:value={color} />
		{#if fieldError('accent_color') !== undefined}
			<p class="text-sm text-error">{fieldError('accent_color')}</p>
		{/if}
	</div>
	<SelectField
		label={t('business_push_hour')}
		bind:value={pushHour}
		options={hours}
		error={fieldError('push_hour')}
	/>
	<p class="-mt-2 text-sm text-ink-muted">{t('business_push_hint')}</p>
	{#if otherError !== null}
		<p role="alert" class="text-sm text-error">{otherError}</p>
	{/if}
</FormCard>

<Card
	as="form"
	radius="xl"
	pad="section"
	class="flex flex-col gap-4 lg:max-w-190"
	onsubmit={(e: SubmitEvent) => {
		e.preventDefault();
		void uploadIcon.run();
	}}
>
	<h2 class="text-xl font-bold text-ink">{t('business_icon')}</h2>
	<p class="text-sm text-ink-muted">{t('business_icon_hint')}</p>
	<IconFileInput
		onpick={(file: File) => {
			iconFile = file;
		}}
	/>
	{#if iconError !== null}
		<p role="alert" class="text-sm text-error">{iconError}</p>
	{/if}
	<div class="lg:w-56">
		{#if iconFile === null}
			<Button variant="strong" type="submit" disabled disabledReason={t('setup_icon_none')}>
				{t('business_icon_upload')}
			</Button>
		{:else if cache.offline}
			<Button variant="strong" type="submit" disabled disabledReason={t('offline_cannot_save')}>
				{t('business_icon_upload')}
			</Button>
		{:else}
			<Button variant="strong" type="submit" busy={iconBusy}>{t('business_icon_upload')}</Button>
		{/if}
	</div>
</Card>
