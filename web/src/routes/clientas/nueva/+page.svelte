<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import type { Path } from '$app/types';
	import type { Client } from '../../../lib/api/schemas';
	import { returnTo } from '../../../lib/booking';
	import { savedDestination } from '../../../lib/clients';
	import { layout } from '../../../lib/layout';
	import { cache } from '../../../lib/cache/store.svelte';
	import ClientForm from '../../../lib/components/ClientForm.svelte';
	import IconButton from '../../../lib/components/ui/IconButton.svelte';
	import { t } from '../../../lib/i18n';

	// "Nueva clienta" (design doc item 76). `volver` is the screen she came from; `savedDestination`
	// decides where saving goes. `returnTo` checked the path is inside the app; `resolve` types a
	// path without its first slash.
	const from = $derived(page.url.searchParams.get('volver'));
	const wide = $derived(layout.current === 'wide');
	const toPath = (id: string): Path => savedDestination(from, wide, id).slice(1) as Path;
	const home = $derived(resolve('/clientas'));
	const back = $derived(from === null ? home : resolve(returnTo(from).slice(1) as Path));
</script>

<div class="flex flex-col gap-4">
	<header class="flex items-center gap-2">
		<IconButton
			variant="plain"
			icon="chevron-left"
			label={t('client_back')}
			href={back}
			class="lg:hidden"
		/>
		<h1 class="text-3xl font-extrabold text-ink">{t('client_new_title')}</h1>
	</header>

	{#if cache.snapshot === null}
		<p role="status" class="px-2 text-base text-ink-muted">{t('clients_loading')}</p>
	{:else}
		<ClientForm
			data={cache.snapshot}
			onsaved={(saved: Client) => {
				void goto(resolve(toPath(saved.id)));
			}}
			oncancel={() => {
				void goto(back);
			}}
		/>
	{/if}
</div>
