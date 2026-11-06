<script lang="ts">
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import type { Snippet } from 'svelte';
	import TextButton from '../../lib/components/ui/TextButton.svelte';
	import SettingsList from '../../lib/components/SettingsList.svelte';
	import { t } from '../../lib/i18n';
	import { layout } from '../../lib/layout';
	import { showSettingsBack } from '../../lib/settingsPage';

	// Ajustes (design doc "Screen layouts"): on wide the section list (280 px) stays on the left
	// and the open section fills the right, with Negocio at `/ajustes`. On compact the list is the
	// screen at `/ajustes` and a section replaces it, under a "Volver a Ajustes" bar. Plantillas
	// sits in the right column and brings its own list and editor split.
	let { children }: { children: Snippet } = $props();

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
			<TextButton variant="back" href={resolve('/ajustes')} class="self-start">
				{t('settings_back')}
			</TextButton>
		{/if}
		{@render children()}
	</div>
{/if}
