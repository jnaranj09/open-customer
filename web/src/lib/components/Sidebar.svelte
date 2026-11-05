<script lang="ts">
	import { resolve } from '$app/paths';
	import { t } from '../i18n';
	import { isActivePath } from '../nav';
	import Icon from './icons/Icon.svelte';
	import { NAV_ITEMS } from './navItems';
	import PushActivate from './PushActivate.svelte';
	import WriteLink from './WriteLink.svelte';

	let {
		name,
		pathname,
		dueCount,
		newDay,
		sync
	}: {
		name: string;
		pathname: string;
		dueCount: number;
		/** The day "Nueva cita" opens on: the day Hoy shows, or today. */
		newDay: string;
		/** The sync line: when the data is from, or that the system is off. */
		sync: string;
	} = $props();

	// The icon is never cached (the manifest rules): offline it cannot load, and a broken image
	// box is worse than none.
	let iconFailed = $state(false);
</script>

<!-- The wide layout's shell (design doc "Screen layouts"): fixed, 256 px, no bottom bar. -->
<aside
	class="glass-panel fixed inset-y-0 left-0 z-30 flex w-64 flex-col gap-6 rounded-none border-y-0 border-l-0 p-6"
>
	<div class="flex items-center gap-3">
		{#if !iconFailed}
			<img
				src="/icons/512.png"
				alt=""
				class="size-10 shrink-0 rounded-md"
				onerror={() => {
					iconFailed = true;
				}}
			/>
		{/if}
		<p class="min-w-0 font-display text-xl font-semibold wrap-break-word text-ink">{name}</p>
	</div>

	<WriteLink href={resolve(`/citas/nueva?dia=${newDay}` as const)} variant="primary">
		<Icon name="plus" />
		{t('appointment_new')}
	</WriteLink>

	<nav aria-label={t('nav_label')} class="flex flex-col gap-1">
		{#each NAV_ITEMS as item (item.href)}
			{@const active = isActivePath(item.href, pathname)}
			<a
				href={resolve(item.href)}
				aria-current={active ? 'page' : undefined}
				class="flex min-h-tap items-center gap-3 rounded-pill px-4 text-sm font-bold {active
					? 'bg-glass-tint text-accent-700'
					: 'text-ink-muted hover:bg-accent-100'}"
			>
				<Icon name={item.icon} />
				{t(item.label)}
				{#if item.href === '/enviar' && dueCount > 0}
					<span class="ml-auto rounded-pill bg-accent px-2 py-0.5 text-xs text-on-accent">
						{dueCount}
					</span>
				{/if}
			</a>
		{/each}
	</nav>

	<div class="mt-auto flex flex-col gap-3">
		<PushActivate />
		<p class="text-sm text-ink-muted">{sync}</p>
	</div>
</aside>
