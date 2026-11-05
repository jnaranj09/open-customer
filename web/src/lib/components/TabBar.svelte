<script lang="ts">
	import { resolve } from '$app/paths';
	import { t } from '../i18n';
	import { isActivePath } from '../nav';
	import Icon from './icons/Icon.svelte';
	import { NAV_ITEMS } from './navItems';

	let { pathname }: { pathname: string } = $props();
</script>

<!-- The floating pill of the mockup's BottomNav, above the home indicator. -->
<nav aria-label={t('nav_label')} class="pb-safe fixed inset-x-0 bottom-0 z-30 px-4">
	<div class="glass-panel mb-6 flex gap-1 rounded-pill p-2">
		{#each NAV_ITEMS as item (item.href)}
			{@const active = isActivePath(item.href, pathname)}
			<a
				href={resolve(item.href)}
				aria-current={active ? 'page' : undefined}
				class="flex min-h-tap flex-1 flex-col items-center justify-center gap-0.5 rounded-pill px-3 py-2 text-xs font-bold {active
					? 'bg-glass-tint text-accent-700'
					: 'text-ink-muted'}"
			>
				<Icon name={item.icon} class="size-5.5" />
				{t(item.tabLabel)}
			</a>
		{/each}
	</div>
</nav>
