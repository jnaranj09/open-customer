<script lang="ts">
	import type { Snippet } from 'svelte';
	import { buttonClasses, type ButtonVariant } from './buttonStyle';

	// A disabled button always says why (ui.md). `busy` needs no reason: its spinner is the reason.
	// The reason shows under the button (`disabledReason`), or, when many buttons share one reason
	// (the cards of a list), once elsewhere on the screen: `reasonId` is that line's id.
	type Props = {
		variant: ButtonVariant;
		busy?: boolean;
		onclick?: () => void;
		type?: 'button' | 'submit';
		children: Snippet;
	} & (
		| { disabled?: false; disabledReason?: undefined; reasonId?: undefined }
		| { disabled: true; disabledReason: string; reasonId?: undefined }
		| { disabled: true; disabledReason?: undefined; reasonId: string }
	);

	let {
		variant,
		busy = false,
		disabled = false,
		disabledReason,
		reasonId: sharedReasonId,
		onclick,
		type = 'button',
		children
	}: Props = $props();

	const ownReasonId = $props.id();
	const reasonId = $derived(sharedReasonId ?? ownReasonId);
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
	{#if disabled && sharedReasonId === undefined}
		<p id={reasonId} class="text-center text-sm text-ink-muted">{disabledReason}</p>
	{/if}
</div>
