<script lang="ts" module>
	/** The .pen TabNav variants: pills on the phone, an underline bar on wide. */
	export type TabNavVariant = 'pill' | 'underline';
</script>

<script lang="ts">
	import { page } from '$app/state';
	import { t } from '../../i18n';
	import { layout } from '../../layout';
	import Card from './Card.svelte';
	import { isCurrent, type Tab } from './tabNav';

	// The tabs of a place (reminders-and-replies design, "Tabs and places"): one URL per tab, so this
	// is a nav with `aria-current`, not an ARIA tablist. The layout picks the variant.
	let { label, tabs }: { label: string; tabs: readonly Tab[] } = $props();

	const variant = $derived<TabNavVariant>(layout.current === 'wide' ? 'underline' : 'pill');
</script>

<nav
	aria-label={label}
	class={variant === 'underline' ? 'flex gap-6 border-b border-glass-edge' : 'flex gap-2'}
>
	{#each tabs as tab (tab.href)}
		{@const current = isCurrent(tab, page.url.pathname)}
		{#if variant === 'underline'}
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
