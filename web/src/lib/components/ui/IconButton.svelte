<script lang="ts" module>
	/** The .pen IconButton variants: `add` (the floating "+"), `plain` (back, previous, next),
	 *  `muted` (the same in the muted ink), `glass` (on a photo), `square` (a row tool: drag handle,
	 *  menu), `accent` (inside a selected chip). */
	export type IconButtonVariant = 'add' | 'plain' | 'muted' | 'glass' | 'square' | 'accent';
</script>

<script lang="ts">
	import type { ResolvedPathname } from '$app/types';
	import type { HTMLButtonAttributes } from 'svelte/elements';
	import Icon from '../icons/Icon.svelte';
	import type { IconName } from '../icons/paths';

	// A round (or square) icon-only control. `label` is its accessible name and tooltip. With
	// `href` it is a link, with `onclick` a button. `inert` shows it faded and inactive (the
	// offline "+"), so a reason must be on screen (the offline banner says it for the "+"). `class` is for position only (`lg:hidden`,
	// `absolute top-1`); other attributes (`disabled`, `aria-expanded`, pointer handlers, `data-*`)
	// go to the button.
	type Props = Omit<
		HTMLButtonAttributes,
		'class' | 'children' | 'title' | 'aria-label' | 'inert'
	> & {
		variant: IconButtonVariant;
		icon: IconName;
		label: string;
		href?: ResolvedPathname;
		inert?: boolean;
		class?: string;
	};

	let {
		variant,
		icon,
		label,
		href,
		inert = false,
		class: placeClass = '',
		...rest
	}: Props = $props();

	const LOOK: Record<IconButtonVariant, string> = {
		add: 'size-14 rounded-pill bg-accent-fill text-on-accent shadow-raised',
		plain: 'size-11 rounded-pill text-ink hover:bg-accent-100',
		muted: 'size-11 rounded-pill text-ink-muted hover:bg-accent-100',
		glass: 'glass-card size-11 rounded-pill text-ink hover:bg-accent-100',
		square: 'size-11 rounded-md text-ink-muted hover:bg-accent-100',
		accent: 'size-11 rounded-pill text-accent-700 hover:bg-accent-300'
	};
	const ICON: Record<IconButtonVariant, string> = {
		add: 'size-7',
		plain: 'size-5.5',
		muted: 'size-4.5',
		glass: 'size-4.5',
		square: 'size-5',
		accent: 'size-4.5'
	};
	const classes = $derived(
		`grid shrink-0 place-items-center ${LOOK[variant]} ${
			variant === 'add' && !inert ? 'hover:opacity-90' : ''
		} ${inert ? 'opacity-60' : ''} ${placeClass}`
	);
</script>

{#if inert}
	<span class={classes} role="img" aria-label={label}
		><Icon name={icon} class={ICON[variant]} /></span
	>
{:else if href !== undefined}
	<a {href} title={label} aria-label={label} class={classes}>
		<Icon name={icon} class={ICON[variant]} />
	</a>
{:else}
	<button type="button" title={label} aria-label={label} {...rest} class={classes}>
		<Icon name={icon} class={ICON[variant]} />
	</button>
{/if}
