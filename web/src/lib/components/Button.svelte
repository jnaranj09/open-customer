<script lang="ts">
	import type { Snippet } from 'svelte';
	import { buttonClasses, type ButtonVariant } from './buttonStyle';

	// A disabled button always says why (ui.md). `busy` needs no reason: its spinner is the reason.
	type Props = {
		variant: ButtonVariant;
		busy?: boolean;
		onclick?: () => void;
		type?: 'button' | 'submit';
		children: Snippet;
	} & (
		{ disabled?: false; disabledReason?: undefined } | { disabled: true; disabledReason: string }
	);

	let {
		variant,
		busy = false,
		disabled = false,
		disabledReason,
		onclick,
		type = 'button',
		children
	}: Props = $props();

	const reasonId = $props.id();
	const off = $derived(disabled || busy);
</script>

<!-- The width belongs to the parent: the wrapper fills it, the button fills the wrapper. -->
<div class="flex w-full flex-col items-center gap-1">
	<button
		{type}
		{onclick}
		disabled={off}
		aria-busy={busy}
		aria-describedby={disabled ? reasonId : undefined}
		class="{buttonClasses(variant)} w-full disabled:cursor-not-allowed disabled:opacity-60"
	>
		{#if busy}
			<span
				class="size-4 animate-spin rounded-pill border-2 border-current border-t-transparent"
				aria-hidden="true"
			></span>
		{/if}
		{@render children()}
	</button>
	{#if disabled}
		<p id={reasonId} class="text-center text-sm text-ink-muted">{disabledReason}</p>
	{/if}
</div>
