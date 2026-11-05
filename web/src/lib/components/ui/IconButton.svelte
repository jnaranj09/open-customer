<script lang="ts" module>
	/** `add` is the .pen IconButton/Add (the floating "+"); `plain` the bare round icon button (back,
	 *  previous, next); `muted` the same in the muted ink; `glass` sits on a photo. */
	export type IconButtonVariant = 'add' | 'plain' | 'muted' | 'glass';
</script>

<script lang="ts">
	import type { ResolvedPathname } from '$app/types';
	import Icon from '../icons/Icon.svelte';
	import type { IconName } from '../icons/paths';

	// A round icon-only control. `label` is its accessible name and tooltip. With `href` it is a
	// link, with `onclick` a button; `disabled` shows it faded and inert (offline "+"), so it must
	// carry a reason near it. `class` is for position only (`lg:hidden`, `absolute top-1`).
	let {
		variant,
		icon,
		label,
		href,
		onclick,
		disabled = false,
		class: placeClass = ''
	}: {
		variant: IconButtonVariant;
		icon: IconName;
		label: string;
		href?: ResolvedPathname;
		onclick?: () => void;
		disabled?: boolean;
		class?: string;
	} = $props();

	const LOOK: Record<IconButtonVariant, string> = {
		add: 'size-14 bg-accent text-on-accent shadow-raised',
		plain: 'size-11 text-ink hover:bg-accent-100',
		muted: 'size-11 text-ink-muted hover:bg-accent-100',
		glass: 'glass-card size-11 text-ink hover:bg-accent-100'
	};
	const ICON: Record<IconButtonVariant, string> = {
		add: 'size-7',
		plain: 'size-5.5',
		muted: 'size-4.5',
		glass: 'size-4.5'
	};
	const classes = $derived(
		`grid place-items-center rounded-pill ${LOOK[variant]} ${
			variant === 'add' && !disabled ? 'hover:bg-accent-700' : ''
		} ${disabled ? 'opacity-60' : ''} ${placeClass}`
	);
</script>

{#if disabled}
	<span class={classes} role="img" aria-label={label}
		><Icon name={icon} class={ICON[variant]} /></span
	>
{:else if href !== undefined}
	<a {href} title={label} aria-label={label} class={classes}>
		<Icon name={icon} class={ICON[variant]} />
	</a>
{:else}
	<button type="button" title={label} aria-label={label} {onclick} class={classes}>
		<Icon name={icon} class={ICON[variant]} />
	</button>
{/if}
