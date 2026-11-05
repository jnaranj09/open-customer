<script lang="ts">
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { setContext, type Snippet } from 'svelte';
	import Icon from '../../lib/components/icons/Icon.svelte';
	import SettingsList from '../../lib/components/SettingsList.svelte';
	import Toast from '../../lib/components/Toast.svelte';
	import { t } from '../../lib/i18n';
	import { layout } from '../../lib/layout';
	import { showSettingsBack } from '../../lib/settingsPage';
	import { createToaster } from '../../lib/toaster.svelte';

	// Ajustes (design doc "Screen layouts"): on wide the section list (280 px) stays on the left
	// and the open section fills the right, with Negocio at `/ajustes`. On compact the list is the
	// screen at `/ajustes` and a section replaces it, under a "Volver a Ajustes" bar. Plantillas
	// sits in the right column and brings its own list and editor split.
	let { children }: { children: Snippet } = $props();

	const toaster = createToaster();
	setContext('settings-toaster', toaster);

	const wide = $derived(layout.current === 'wide');
	const onRoot = $derived(page.url.pathname.replace(/\/$/, '') === resolve('/ajustes'));
</script>

{#snippet title()}
	<svelte:element this={wide ? 'p' : 'h1'} class="text-3xl font-extrabold text-ink">
		{t('settings_title')}
	</svelte:element>
{/snippet}

{#if wide}
	<div class="flex items-start gap-8">
		<div class="flex w-70 shrink-0 flex-col gap-4">
			<div class="flex min-h-12 items-center">{@render title()}</div>
			<SettingsList />
		</div>
		<div class="min-w-0 flex-1">{@render children()}</div>
	</div>
{:else if onRoot}
	<div class="flex flex-col gap-4">
		<div class="flex min-h-12 items-center">{@render title()}</div>
		<SettingsList />
	</div>
{:else}
	<div class="flex flex-col gap-2">
		{#if showSettingsBack(page.url.pathname)}
			<a
				href={resolve('/ajustes')}
				class="inline-flex min-h-tap items-center gap-1 self-start rounded-pill pr-4 text-sm font-bold text-accent-700 hover:bg-accent-100"
			>
				<Icon name="chevron-left" class="size-5" />
				{t('settings_back')}
			</a>
		{/if}
		{@render children()}
	</div>
{/if}

{#if toaster.current !== null}
	{#key toaster.current.n}
		<Toast
			message={toaster.current.message}
			{...toaster.current.action === undefined
				? {}
				: { actionLabel: toaster.current.action.label, onaction: toaster.current.action.run }}
		/>
	{/key}
{/if}
