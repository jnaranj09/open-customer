<script lang="ts">
	import { cache } from '../../lib/cache/store.svelte';
	import { clock } from '../../lib/clock.svelte';
	import QuoteForm from '../../lib/components/QuoteForm.svelte';
	import { t } from '../../lib/i18n';

	// The Cotizar tab of Servicios (quick-quotes design, item 23). The form mounts once the catalog
	// is loaded, so its draft always restores against it. `today` is the day Convertir en cita gives
	// Nueva cita.
	const data = $derived(cache.snapshot);
</script>

{#if data === null}
	<p role="status" class="px-2 text-base text-ink-muted">{t('services_loading')}</p>
{:else}
	<QuoteForm
		services={data.services.filter((s) => !s.archived)}
		settings={data.settings}
		today={clock.today(data.settings.timezone)}
	/>
{/if}
