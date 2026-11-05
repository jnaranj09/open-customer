<script lang="ts">
	import type { Snippet } from 'svelte';
	import { page } from '$app/state';
	import ClientList from '../../lib/components/ClientList.svelte';
	import { layout } from '../../lib/layout';

	// Clientas is master-detail (design doc "Screen layouts"): on wide the list (380 px) stays on the
	// left while the client, the form or "Elige una clienta" fills the right. On compact the list is
	// the screen at /clientas, and a client or a form replaces it. The list is always mounted, only
	// hidden on compact, so her search and "Ver archivadas" survive a visit to a client.
	let { children }: { children: Snippet } = $props();

	const onList = $derived(page.url.pathname.replace(/\/$/, '') === '/clientas');
	const wide = $derived(layout.current === 'wide');
</script>

<div class="flex items-start gap-8">
	<div class="min-w-0 {wide ? 'w-95 shrink-0' : 'w-full'} {wide || onList ? '' : 'hidden'}">
		<ClientList selectedId={page.params.id ?? null} />
	</div>
	{#if wide || !onList}
		<div class="min-w-0 flex-1">{@render children()}</div>
	{/if}
</div>
