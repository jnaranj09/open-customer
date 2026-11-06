<script lang="ts" module>
	/** Inner spacing: `md` for a note or a small form, `trail` for a line with a trailing action. */
	export type CalloutPad = 'md' | 'trail';
	export type CalloutElement = 'div' | 'p' | 'form';
</script>

<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { HTMLAttributes } from 'svelte/elements';

	// The .pen Callout: a tinted box with no glass edge, for a note inside a card or a sheet (a
	// warning, a restored draft, the booking prompt, an inline form). `class` is layout and text
	// only; padding comes from `pad`.
	type Props = Omit<HTMLAttributes<HTMLElement>, 'class' | 'children'> & {
		pad?: CalloutPad;
		as?: CalloutElement;
		class?: string;
		children: Snippet;
	};

	let { pad = 'md', as = 'div', class: layoutClass = '', children, ...rest }: Props = $props();

	const PAD: Record<CalloutPad, string> = { md: 'px-4 py-3', trail: 'py-1 pr-1 pl-4' };
</script>

<svelte:element
	this={as}
	{...rest}
	class="rounded-md bg-glass-tint {PAD[pad]} text-ink {layoutClass}"
>
	{@render children()}
</svelte:element>
