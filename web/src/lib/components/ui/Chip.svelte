<script lang="ts" module>
	/** The .pen Chip variants: `selected` is the picked one of a set. */
	export type ChipVariant = 'default' | 'selected';
</script>

<script lang="ts">
	import type { Snippet } from 'svelte';

	// A toggle chip: one of a small set of choices. `aria-pressed` follows the variant. `trailing`
	// is a second control inside the same pill (a minus that takes one copy away).
	let {
		variant,
		onclick,
		trailing,
		children
	}: {
		variant: ChipVariant;
		onclick: () => void;
		trailing?: Snippet;
		children: Snippet;
	} = $props();

	const SURFACE: Record<ChipVariant, string> = {
		default: 'border-glass-edge bg-glass-strong',
		selected: 'border-accent bg-accent-100'
	};
	const INK: Record<ChipVariant, string> = {
		default: 'text-ink hover:bg-accent-100',
		selected: 'text-accent-700'
	};
</script>

{#if trailing === undefined}
	<button
		type="button"
		aria-pressed={variant === 'selected'}
		class="min-h-tap rounded-pill border px-4 text-sm font-bold {SURFACE[variant]} {INK[variant]}"
		{onclick}
	>
		{@render children()}
	</button>
{:else}
	<div class="inline-flex min-h-tap items-center rounded-pill border {SURFACE[variant]}">
		<button
			type="button"
			aria-pressed={variant === 'selected'}
			class="min-h-tap rounded-pill px-4 text-sm font-bold hover:bg-accent-100 {INK[variant]}"
			{onclick}
		>
			{@render children()}
		</button>
		{@render trailing()}
	</div>
{/if}
