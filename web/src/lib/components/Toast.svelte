<script lang="ts">
	import Card from './ui/Card.svelte';
	import { UNDO_MS } from '../undo';

	let {
		message,
		actionLabel,
		onaction,
		ms = UNDO_MS
	}: { message: string; actionLabel?: string; onaction?: () => void; ms?: number } = $props();

	// Shown on mount, gone after `ms` or the action. A new message needs a new toast: the caller
	// wraps it in `{#key}`. The default `ms` is `UNDO_MS`, the delay of `delayed` (lib/undo.ts).
	//
	// Stacking: a sheet (BottomSheet, z-50) rises from the bottom or sits centered, so the toast
	// lives at the top of the screen on a layer above it (z-60): it never covers a sheet's buttons,
	// and a toast raised while a sheet is open (an "Omitir" undo) stays visible and tappable.
	let gone = $state(false);

	$effect(() => {
		const timer = setTimeout(() => {
			gone = true;
		}, ms);
		return () => {
			clearTimeout(timer);
		};
	});

	function act(): void {
		gone = true;
		onaction?.();
	}
</script>

{#if !gone}
	<div
		role="status"
		class="pointer-events-none fixed inset-x-0 top-0 z-60 flex justify-center px-4 pt-safe"
	>
		<Card
			variant="strong"
			radius="pill"
			pad="trail"
			class="pointer-events-auto mt-4 flex min-h-tap items-center gap-4 text-ink"
		>
			<p class="text-sm">{message}</p>
			{#if actionLabel !== undefined}
				<button
					type="button"
					onclick={act}
					class="min-h-tap rounded-pill px-4 text-sm font-bold text-accent-700 hover:bg-accent-100"
				>
					{actionLabel}
				</button>
			{/if}
		</Card>
	</div>
{/if}
