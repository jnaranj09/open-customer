<script lang="ts">
	import TextButton from './ui/TextButton.svelte';
	import Card from './ui/Card.svelte';
	import { LIVE_LAYER, toBody } from '../inert';
	import type { ToastState } from '../toaster.svelte';
	import { UNDO_MS } from '../undo';

	let { toast, ms = UNDO_MS }: { toast: ToastState | null; ms?: number } = $props();

	// The app's one toast host, mounted once by the root layout. Each message (a new `n`) shows,
	// then goes after `ms` or the action. The default `ms` is `UNDO_MS`, the delay of `delayed`
	// (lib/undo.ts).
	//
	// Screen readers: the host is the polite live region (`role="status"`), mounted empty with the
	// app and always there, so each message is put into a region that already exists. Some screen
	// readers skip a live region that is inserted with its text already in it. Each message is a new node (`{#key}`), so a repeat of the same text ("Copiado" twice) is
	// announced again. The message is read once: nothing else on the screen repeats it.
	//
	// Stacking: a sheet (BottomSheet, z-50) rises from the bottom or sits centered, so the toast
	// lives at the top of the screen on a layer above it (z-60): it never covers a sheet's buttons,
	// and a toast raised while a sheet is open (an "Omitir" undo) stays visible and tappable. For
	// that it is its own live layer of <body>, outside the page a sheet makes inert.
	//
	// Padding (mobile-polish design, section 5): even (`message`) for a toast that is only text, as
	// in the design; `trail` leaves room on the right for the action button.
	let goneN = $state<number | null>(null);

	$effect(() => {
		const n = toast?.n;
		if (n === undefined) {
			return;
		}
		const timer = setTimeout(() => {
			goneN = n;
		}, ms);
		return () => {
			clearTimeout(timer);
		};
	});

	const shown = $derived(toast !== null && toast.n !== goneN ? toast : null);

	function act(n: number, run: () => void): void {
		goneN = n;
		run();
	}
</script>

<div
	{@attach toBody}
	{...{ [LIVE_LAYER]: '' }}
	role="status"
	aria-live="polite"
	class="pointer-events-none fixed inset-x-0 top-0 z-60 flex justify-center px-4 pt-safe"
>
	{#if shown !== null}
		{#key shown.n}
			<Card
				variant="solid"
				radius="pill"
				pad={shown.action === undefined ? 'message' : 'trail'}
				class="pointer-events-auto mt-4 flex min-h-tap items-center gap-4 text-ink"
			>
				<p class="text-sm">{shown.message}</p>
				{#if shown.action !== undefined}
					{@const n = shown.n}
					{@const run = shown.action.run}
					<TextButton
						variant="accent"
						onclick={() => {
							act(n, run);
						}}>{shown.action.label}</TextButton
					>
				{/if}
			</Card>
		{/key}
	{/if}
</div>
