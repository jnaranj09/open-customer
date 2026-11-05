<script lang="ts" module>
	/** `tab` is the .pen NavItem (icon over a short label, in the bottom bar); `row` is the same
	 *  place as a sidebar row (icon beside the label, with an optional count). */
	export type NavItemVariant = 'tab' | 'row';
</script>

<script lang="ts">
	import type { ResolvedPathname } from '$app/types';
	import Icon from '../icons/Icon.svelte';
	import type { IconName } from '../icons/paths';

	let {
		variant,
		href,
		icon,
		label,
		active,
		count = 0
	}: {
		variant: NavItemVariant;
		href: ResolvedPathname;
		icon: IconName;
		label: string;
		active: boolean;
		/** Shown as a badge on a `row` when above zero. */
		count?: number;
	} = $props();
</script>

{#if variant === 'tab'}
	<a
		{href}
		aria-current={active ? 'page' : undefined}
		class="flex min-h-tap flex-1 flex-col items-center justify-center gap-0.5 rounded-pill px-3 py-2 text-xs font-bold {active
			? 'bg-glass-tint text-accent-700'
			: 'text-ink-muted'}"
	>
		<Icon name={icon} class="size-5.5" />
		{label}
	</a>
{:else}
	<a
		{href}
		aria-current={active ? 'page' : undefined}
		class="flex min-h-tap items-center gap-3 rounded-pill px-4 text-sm font-bold {active
			? 'bg-glass-tint text-accent-700'
			: 'text-ink-muted hover:bg-accent-100'}"
	>
		<Icon name={icon} />
		{label}
		{#if count > 0}
			<span class="ml-auto rounded-pill bg-accent px-2 py-0.5 text-xs text-on-accent">
				{count}
			</span>
		{/if}
	</a>
{/if}
