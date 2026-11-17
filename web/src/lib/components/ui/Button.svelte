<script lang="ts">
	import type { Snippet } from 'svelte';
	import BusyDots from './BusyDots.svelte';
	import { buttonClasses, type ButtonVariant } from './buttonStyle';

	// A disabled button always says why (ui.md). `busy` needs no reason: its dots are the reason.
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
		class="{buttonClasses(variant)} relative w-full {disabled
			? 'cursor-not-allowed opacity-60'
			: busy
				? 'cursor-progress'
				: ''}"
	>
		<!-- Busy: the label stays (its width and the accessible name), unseen; the dots sit over it,
		     so nothing moves and the button keeps its full color (faded means disabled). -->
		<span class="inline-flex items-center justify-center gap-2 {busy ? 'opacity-0' : ''}">
			{@render children()}
		</span>
		{#if busy}
			<span class="absolute inset-0 flex items-center justify-center">
				<BusyDots />
			</span>
		{/if}
	</button>
	{#if disabled && sharedReasonId === undefined}
		<p id={reasonId} class="text-center text-sm text-ink-muted">{disabledReason}</p>
	{/if}
</div>
