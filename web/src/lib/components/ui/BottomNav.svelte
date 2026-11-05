<script lang="ts">
	import { resolve } from '$app/paths';
	import { t } from '../../i18n';
	import { isActivePath } from '../../nav';
	import { NAV_ITEMS } from '../navItems';
	import NavItem from './NavItem.svelte';

	let { pathname }: { pathname: string } = $props();
</script>

<!-- The floating pill of the mockup's BottomNav, above the home indicator. -->
<nav aria-label={t('nav_label')} class="pb-safe fixed inset-x-0 bottom-0 z-30 px-4">
	<div class="glass-panel mb-6 flex gap-1 rounded-pill p-2">
		{#each NAV_ITEMS as item (item.href)}
			<NavItem
				variant="tab"
				href={resolve(item.href)}
				icon={item.icon}
				label={t(item.tabLabel)}
				active={isActivePath(item.href, pathname)}
			/>
		{/each}
	</div>
</nav>
