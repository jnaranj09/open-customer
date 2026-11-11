<script lang="ts" module>
	/** The .pen LabelChip variants: `warning` when the item has a week or less left. */
	export type LabelChipVariant = 'normal' | 'warning';
</script>

<script lang="ts">
	import { resolve } from '$app/paths';
	import Icon from '../icons/Icon.svelte';

	// The label of a due item (reminders-and-replies design, "Row label"): where it comes from, which
	// rule, which message and how long it stays. A link to the source appointment when
	// `appointmentId` is set (online), plain text otherwise (offline the appointment may not be
	// cached). It sits next to the row's buttons, never inside one.
	let {
		variant,
		text,
		appointmentId = null
	}: { variant: LabelChipVariant; text: string; appointmentId?: string | null } = $props();

	const LOOK: Record<LabelChipVariant, string> = {
		normal: 'border-glass-edge text-ink-muted',
		warning: 'border-warning text-warning'
	};
	const icon = $derived(variant === 'warning' ? 'hourglass' : 'calendar');
	const classes = $derived(
		`inline-flex max-w-full min-h-tap items-center gap-1.5 rounded-pill border bg-glass-strong px-3 py-1.5 text-sm font-semibold ${LOOK[variant]}`
	);
</script>

{#if appointmentId === null}
	<span class={classes}>
		<Icon name={icon} class="size-3.5 shrink-0" />
		<span class="min-w-0 break-words">{text}</span>
	</span>
{:else}
	<a href={resolve('/citas/[id]', { id: appointmentId })} class="{classes} hover:bg-accent-100">
		<Icon name={icon} class="size-3.5 shrink-0" />
		<span class="min-w-0 break-words">{text}</span>
	</a>
{/if}
