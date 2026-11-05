<script lang="ts">
	import type { Snippet } from 'svelte';

	// The stand-alone screens (setup, login, new password): flat on the glow background on a phone
	// (the mockup's Login frame), a centered 440 px glass card on a wide screen.
	let { title, caption, children }: { title: string; caption?: string; children: Snippet } =
		$props();

	// The icon is public, but offline or before the first load it may not arrive.
	let iconFailed = $state(false);
</script>

<main class="grid min-h-dvh place-items-center px-6 py-14">
	<div class="flex w-full max-w-110 flex-col gap-6 lg:glass-panel lg:rounded-lg lg:p-10">
		{#if !iconFailed}
			<img
				src="/icons/512.png"
				alt=""
				class="size-18 rounded-pill object-cover shadow-raised"
				onerror={() => {
					iconFailed = true;
				}}
			/>
		{/if}
		<div class="flex flex-col gap-1">
			<h1 class="text-3xl font-extrabold text-ink">{title}</h1>
			{#if caption !== undefined}
				<p class="text-base text-ink-muted">{caption}</p>
			{/if}
		</div>
		{@render children()}
	</div>
</main>
