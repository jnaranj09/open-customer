<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import type { Path } from '$app/types';
	import type { Client } from '../../../lib/api/schemas';
	import { returnTo } from '../../../lib/booking';
	import { cache } from '../../../lib/cache/store.svelte';
	import ClientForm from '../../../lib/components/ClientForm.svelte';
	import Icon from '../../../lib/components/icons/Icon.svelte';
	import { t } from '../../../lib/i18n';

	// "Nueva clienta" (design doc item 76). `volver` is the screen she came from; without it the new
	// client's own page opens. `returnTo` checked the path is inside the app; `resolve` types a
	// path without its first slash.
	const from = $derived(page.url.searchParams.get('volver'));
	const back = $derived(resolve(returnTo(from).slice(1) as Path));
	const home = $derived(resolve('/clientas'));
</script>

<div class="flex flex-col gap-4">
	<header class="flex items-center gap-2">
		<a
			href={from === null ? home : back}
			title={t('client_back')}
			aria-label={t('client_back')}
			class="grid size-11 place-items-center rounded-pill text-ink hover:bg-accent-100 lg:hidden"
		>
			<Icon name="chevron-left" class="size-5.5" />
		</a>
		<h1 class="font-display text-3xl font-semibold text-ink">{t('client_new_title')}</h1>
	</header>

	{#if cache.snapshot === null}
		<p role="status" class="px-2 text-base text-ink-muted">{t('clients_loading')}</p>
	{:else}
		<ClientForm
			data={cache.snapshot}
			onsaved={(saved: Client) => {
				void goto(from === null ? resolve('/clientas/[id]', { id: saved.id }) : back);
			}}
			oncancel={() => {
				void goto(from === null ? home : back);
			}}
		/>
	{/if}
</div>
