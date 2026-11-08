<script lang="ts">
	import type { Snippet } from 'svelte';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import Card from '../../lib/components/ui/Card.svelte';
	import { t } from '../../lib/i18n';
	import { layout } from '../../lib/layout';
	import { provideHeaderAction } from './header.svelte';

	// Servicios (quick-quotes design, section 5), a top-level place: the heading and, at its right
	// on wide, the action its page registered through `useHeaderAction`. On the phone the slot is
	// empty (no row is drawn): the page uses the floating "+". Under it, the two tab links: pills on
	// the phone, an underline bar on wide. They are two URLs, so this is a nav, not an ARIA tablist.
	let { children }: { children: Snippet } = $props();

	const action = provideHeaderAction();
	const TABS = [
		{ href: resolve('/servicios'), label: 'services_tab_quote' },
		{ href: resolve('/servicios/catalogo'), label: 'services_tab_catalog' }
	] as const;
	const wide = $derived(layout.current === 'wide');
</script>

<div class="flex flex-col gap-4">
	<header class="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
		<h1 class="flex min-h-12 items-center text-3xl font-extrabold text-ink">
			{t('services_title')}
		</h1>
		{#if action.snippet !== null}
			<div class="w-44 shrink-0">{@render action.snippet()}</div>
		{/if}
	</header>
	<!-- An exact match: /servicios is a prefix of /servicios/catalogo. -->
	<nav
		aria-label={t('services_tabs_label')}
		class={wide ? 'flex gap-6 border-b border-glass-edge' : 'flex gap-2'}
	>
		{#each TABS as tab (tab.href)}
			{@const current = page.url.pathname === tab.href}
			{#if wide}
				<a
					href={tab.href}
					aria-current={current ? 'page' : undefined}
					class="-mb-px flex min-h-tap items-center border-b-2 px-1 text-sm font-bold {current
						? 'border-accent text-accent-text'
						: 'border-transparent text-ink-muted hover:text-ink'}">{t(tab.label)}</a
				>
			{:else}
				<Card
					as="a"
					href={tab.href}
					pad="note"
					radius="pill"
					selected={current}
					aria-current={current ? 'page' : undefined}
					class="flex min-h-tap flex-1 items-center justify-center text-sm font-bold"
					>{t(tab.label)}</Card
				>
			{/if}
		{/each}
	</nav>
	{@render children()}
</div>
