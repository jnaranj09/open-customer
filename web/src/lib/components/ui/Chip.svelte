<script lang="ts" module>
	/** The .pen Chip variants: `selected` is the picked one of a set. */
	export type ChipVariant = 'default' | 'selected';
</script>

<script lang="ts">
	import type { Snippet } from 'svelte';

	// A toggle chip: one of a small set of choices. `aria-pressed` follows the variant.
	let {
		variant,
		onclick,
		children
	}: { variant: ChipVariant; onclick: () => void; children: Snippet } = $props();
</script>

<button
	type="button"
	aria-pressed={variant === 'selected'}
	class="min-h-tap rounded-pill border px-4 text-sm font-bold {variant === 'selected'
		? 'border-accent bg-accent-100 text-accent-700'
		: 'border-glass-edge bg-glass-strong text-ink hover:bg-accent-100'}"
	{onclick}
>
	{@render children()}
</button>
