<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { onMount } from 'svelte';
	import { api } from '../../lib/api/app';
	import { errorText } from '../../lib/api/errors';
	import AuthCard from '../../lib/components/AuthCard.svelte';
	import Button from '../../lib/components/Button.svelte';
	import ColorPicker from '../../lib/components/ColorPicker.svelte';
	import InstallGuide from '../../lib/components/InstallGuide.svelte';
	import SelectField from '../../lib/components/SelectField.svelte';
	import TextField from '../../lib/components/TextField.svelte';
	import { applyAccent, DEFAULT_ACCENT } from '../../lib/color';
	import { t } from '../../lib/i18n';
	import { needsInstallGuide } from '../../lib/install';
	import { toIconPngs } from '../../lib/photos';
	import {
		countryOptions,
		currencyOptions,
		stepForField,
		STEPS,
		timeZoneOptions,
		tokenFromHash,
		type Option,
		type WizardStep
	} from '../../lib/setup';
	import { submitOnce } from '../../lib/submit';

	const token = $derived(tokenFromHash(page.url.hash));

	const browserZone = Intl.DateTimeFormat().resolvedOptions().timeZone;
	// Null when the browser cannot list them: those two fields become text boxes.
	const zones = timeZoneOptions(Intl, browserZone);
	const currencies = currencyOptions(Intl);
	let countries = $state<Option[]>([]);
	let optionsError = $state<string | null>(null);

	let step = $state<WizardStep>('business');
	let name = $state('');
	let timezone = $state(browserZone);
	let currency = $state('');
	let country = $state('');
	let color = $state(DEFAULT_ACCENT);
	let username = $state('');
	let password = $state('');
	let iconFile = $state<File | null>(null);

	let stepError = $state<string | null>(null);
	let busy = $state(false);

	const index = $derived(STEPS.indexOf(step));
	const creating = $derived(step === 'password');

	onMount(() => {
		void api.setupOptions().then((result) => {
			if (result.ok) {
				countries = countryOptions(result.data.countries, Intl);
			} else {
				optionsError = errorText(result.error);
			}
		});
	});

	// Shows the color as she picks it.
	$effect(() => {
		applyAccent(document.documentElement, color);
	});

	function go(to: WizardStep, error: string | null = null): void {
		step = to;
		stepError = error;
	}

	/** The value the current step asks for, for the "is it filled" check. */
	function filled(): boolean {
		switch (step) {
			case 'business':
				return name.trim() !== '';
			case 'timezone':
				return timezone.trim() !== '';
			case 'currency':
				return currency.trim() !== '';
			case 'country':
				return country !== '';
			case 'username':
				return username.trim() !== '';
			case 'password':
				return password !== '';
			case 'color':
			case 'icon':
			case 'install':
				return true;
		}
	}

	const create = submitOnce(
		async () => {
			if (token === null) {
				return;
			}
			const result = await api.setup({
				token,
				username,
				password,
				settings: {
					name: name.trim(),
					timezone: timezone.trim(),
					locale: 'es',
					currency: currency.trim().toUpperCase(),
					country_code: country,
					accent_color: color
				}
			});
			if (result.ok) {
				go('icon');
				return;
			}
			const fields = result.error.kind === 'problem' ? Object.keys(result.error.fields) : [];
			const back = fields.map(stepForField).find((s) => s !== null) ?? null;
			go(back ?? step, errorText(result.error));
		},
		(pending) => {
			busy = pending;
		}
	);

	async function next(): Promise<void> {
		if (!filled()) {
			stepError = step === 'country' ? t('field_choose') : t('field_required');
			return;
		}
		stepError = null;
		if (step === 'password') {
			await create.run();
			return;
		}
		go(STEPS[index + 1] ?? 'install');
	}

	const upload = submitOnce(
		async () => {
			if (iconFile === null) {
				return;
			}
			stepError = null;
			let icons;
			try {
				icons = await toIconPngs(iconFile);
			} catch {
				console.warn('code=icon_decode_failed');
				stepError = t('setup_icon_unreadable');
				return;
			}
			const result = await api.uploadIcon(icons);
			if (result.ok) {
				go('install');
			} else {
				stepError = errorText(result.error);
			}
		},
		(pending) => {
			busy = pending;
		}
	);

	function standalone(): boolean {
		return (
			(navigator as Navigator & { standalone?: boolean }).standalone === true ||
			window.matchMedia('(display-mode: standalone)').matches
		);
	}
	const showGuide = $derived(
		step === 'install' && needsInstallGuide(navigator.userAgent, standalone())
	);

	function finish(): void {
		// The layout asks the server again on the new path and shows the app.
		void goto(resolve('/'));
	}
</script>

{#snippet back()}
	{#if index > 0}
		<Button
			variant="secondary"
			onclick={() => {
				go(STEPS[index - 1] ?? 'business');
			}}>{t('form_back')}</Button
		>
	{/if}
{/snippet}

<AuthCard
	title={t('setup_title')}
	caption={t('setup_progress', { n: index + 1, total: STEPS.length })}
>
	{#if token === null}
		<p role="alert" class="text-base text-error">{t('setup_no_token')}</p>
	{:else if step === 'icon'}
		<form
			class="flex flex-col gap-4"
			onsubmit={(e) => {
				e.preventDefault();
				void upload.run();
			}}
		>
			<h2 class="text-xl font-bold text-ink">{t('setup_icon_title')}</h2>
			<p class="text-base text-ink-muted">{t('setup_icon_hint')}</p>
			<label class="flex flex-col gap-1.5 text-sm font-bold text-ink">
				{t('setup_icon_label')}
				<input
					type="file"
					accept="image/*"
					onchange={(e) => {
						iconFile = e.currentTarget.files?.[0] ?? null;
					}}
					class="min-h-tap w-full rounded-sm border border-glass-edge bg-glass-strong p-2 text-base font-normal"
				/>
			</label>
			{#if stepError !== null}
				<p role="alert" class="text-sm text-error">{stepError}</p>
			{/if}
			{#if iconFile === null}
				<Button variant="primary" type="submit" disabled disabledReason={t('setup_icon_none')}
					>{t('setup_icon_upload')}</Button
				>
			{:else}
				<Button variant="primary" type="submit" {busy}>{t('setup_icon_upload')}</Button>
			{/if}
			<Button
				variant="secondary"
				onclick={() => {
					go('install');
				}}>{t('setup_icon_skip')}</Button
			>
		</form>
	{:else if step === 'install'}
		<div class="flex flex-col gap-4">
			{#if showGuide}
				<InstallGuide />
			{:else}
				<h2 class="text-xl font-bold text-ink">{t('setup_done_title')}</h2>
				<p class="text-base text-ink-muted">{t('setup_done_body')}</p>
			{/if}
			<Button variant="primary" onclick={finish}>{t('setup_enter')}</Button>
		</div>
	{:else}
		<form
			class="flex flex-col gap-4"
			onsubmit={(e) => {
				e.preventDefault();
				void next();
			}}
		>
			{#if step === 'business'}
				<TextField
					label={t('setup_business_label')}
					bind:value={name}
					autocomplete="organization"
					error={stepError ?? undefined}
				/>
			{:else if step === 'timezone'}
				{#if zones === null}
					<TextField
						label={t('setup_timezone_label')}
						bind:value={timezone}
						hint={t('setup_timezone_hint')}
						error={stepError ?? undefined}
					/>
				{:else}
					<SelectField
						label={t('setup_timezone_label')}
						bind:value={timezone}
						options={zones}
						error={stepError ?? undefined}
					/>
				{/if}
			{:else if step === 'currency'}
				{#if currencies === null}
					<TextField
						label={t('setup_currency_label')}
						bind:value={currency}
						hint={t('setup_currency_hint')}
						autocapitalize="characters"
						error={stepError ?? undefined}
					/>
				{:else}
					<SelectField
						label={t('setup_currency_label')}
						bind:value={currency}
						options={currencies}
						error={stepError ?? undefined}
					/>
				{/if}
			{:else if step === 'country'}
				<SelectField
					label={t('setup_country_label')}
					bind:value={country}
					options={countries}
					error={stepError ?? optionsError ?? undefined}
				/>
			{:else if step === 'color'}
				<h2 class="text-xl font-bold text-ink">{t('setup_color_title')}</h2>
				<ColorPicker bind:value={color} />
				{#if stepError !== null}
					<p role="alert" class="text-sm text-error">{stepError}</p>
				{/if}
			{:else if step === 'username'}
				<TextField
					label={t('setup_username_label')}
					bind:value={username}
					autocomplete="username"
					autocapitalize="off"
					autocorrect="off"
					hint={t('setup_username_hint')}
					error={stepError ?? undefined}
				/>
			{:else}
				<TextField
					label={t('setup_password_label')}
					type="password"
					bind:value={password}
					autocomplete="new-password"
					hint={t('setup_password_hint')}
					error={stepError ?? undefined}
				/>
			{/if}
			<Button variant="primary" type="submit" {busy}>
				{creating ? t('setup_create') : t('form_next')}
			</Button>
			{@render back()}
		</form>
	{/if}
</AuthCard>
