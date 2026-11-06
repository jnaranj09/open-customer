<script lang="ts">
	import { resolve } from '$app/paths';
	import { t } from '../i18n';
	import { isActivePath } from '../nav';
	import BusinessIcon from './BusinessIcon.svelte';
	import Icon from './icons/Icon.svelte';
	import Card from './ui/Card.svelte';
	import NavItem from './ui/NavItem.svelte';
	import { NAV_ITEMS } from './navItems';
	import PushActivate from './PushActivate.svelte';
	import WriteLink from './WriteLink.svelte';

	let {
		name,
		customIcon,
		pathname,
		dueCount,
		newDay,
		sync
	}: {
		name: string;
		/** Whether an icon was uploaded; undefined before the cache loads (see BusinessIcon). */
		customIcon: boolean | undefined;
		pathname: string;
		dueCount: number;
		/** The day "Nueva cita" opens on: the day Hoy shows, or today. */
		newDay: string;
		/** The sync line: when the data is from, or that the system is off. */
		sync: string;
	} = $props();
</script>

<!-- The wide layout's shell (design doc "Screen layouts"): fixed, 256 px, no bottom bar. -->
<Card
	as="aside"
	variant="strong"
	radius="none"
	pad="lg"
	class="fixed inset-y-0 left-0 z-30 flex w-64 flex-col gap-6 border-y-0 border-l-0"
>
	<!-- The name gets the sidebar's full width under the icon, and a word too long for it breaks
	     at a syllable with a hyphen (the page is lang="es"), not at any letter. -->
	<div class="flex flex-col gap-3">
		<BusinessIcon custom={customIcon} class="size-10 shrink-0 rounded-md" />
		<p class="text-xl font-extrabold hyphens-auto text-ink">{name}</p>
	</div>

	<WriteLink href={resolve(`/citas/nueva?dia=${newDay}` as const)} variant="strong">
		<Icon name="plus" />
		{t('appointment_new')}
	</WriteLink>

	<nav aria-label={t('nav_label')} class="flex flex-col gap-1">
		{#each NAV_ITEMS as item (item.href)}
			<NavItem
				variant="row"
				href={resolve(item.href)}
				icon={item.icon}
				label={t(item.label)}
				active={isActivePath(item.href, pathname)}
				count={item.href === '/enviar' ? dueCount : 0}
			/>
		{/each}
	</nav>

	<div class="mt-auto flex flex-col gap-3">
		<PushActivate />
		<p class="text-sm text-ink-muted">{sync}</p>
	</div>
</Card>
