<script lang="ts">
	import { api } from '../../../lib/api/app';
	import { cache } from '../../../lib/cache/store.svelte';
	import Button from '../../../lib/components/Button.svelte';
	import { buttonClasses } from '../../../lib/components/buttonStyle';
	import { t } from '../../../lib/i18n';

	// Ajustes → Descargar mis datos (design doc item 77, CUJ 8): a plain download link, so the
	// browser saves the streamed ZIP itself. The service worker sends everything under `/api/` to
	// the network, so it never answers this request. `rel="external"` only keeps the router and the
	// lint rule off the link: it is an API file, not an app route. A link cannot be switched off, so
	// offline the page shows a disabled button that says why.
</script>

<div class="flex max-w-190 flex-col gap-4">
	<header class="flex min-h-12 items-center">
		<h1 class="text-3xl font-extrabold text-ink">{t('export_title')}</h1>
	</header>
	<div class="glass-card flex flex-col gap-4 rounded-lg p-5">
		<p class="text-base text-ink-muted">{t('export_body')}</p>
		<div class="sm:w-64">
			{#if cache.offline}
				<Button variant="primary" disabled disabledReason={t('export_offline')}>
					{t('export_button')}
				</Button>
			{:else}
				<a href={api.exportUrl()} download rel="external" class="{buttonClasses('primary')} w-full">
					{t('export_button')}
				</a>
			{/if}
		</div>
	</div>
</div>
