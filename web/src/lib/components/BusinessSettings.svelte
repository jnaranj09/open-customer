<script lang="ts">
	import { cache } from '../cache/store.svelte';
	import { t } from '../i18n';
	import BusinessForm from './BusinessForm.svelte';

	// Ajustes → Negocio (design doc item 77): the page title, then the form once the cached
	// settings are there. It is also what `/ajustes` shows on wide.
</script>

<div class="flex flex-col gap-4">
	<header class="flex min-h-12 items-center">
		<h1 class="font-display text-3xl font-semibold text-ink">{t('business_title')}</h1>
	</header>
	{#if cache.snapshot === null}
		<p role="status" class="px-2 text-base text-ink-muted">{t('business_loading')}</p>
	{:else}
		<BusinessForm settings={cache.snapshot.settings} />
	{/if}
</div>
