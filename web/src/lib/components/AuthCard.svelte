<script lang="ts">
	import type { Snippet } from 'svelte';
	import BusinessIcon from './BusinessIcon.svelte';

	// The stand-alone screens (setup, login, new password): flat on the glow background on a phone
	// (the mockup's Login frame), a centered 440 px glass card on a wide screen. `customIcon` is
	// whether an icon was uploaded (see BusinessIcon); setup knows it, login reads the cache.
	let {
		title,
		caption,
		customIcon,
		children
	}: { title: string; caption?: string; customIcon: boolean | undefined; children: Snippet } =
		$props();
</script>

<main class="grid min-h-dvh place-items-center px-6 py-14">
	<div class="flex w-full max-w-110 flex-col gap-6 lg:glass-panel lg:rounded-lg lg:p-10">
		<BusinessIcon custom={customIcon} class="size-18 self-center rounded-pill shadow-raised" />
		<div class="flex flex-col gap-1">
			<h1 class="text-3xl font-extrabold text-ink">{title}</h1>
			{#if caption !== undefined}
				<p class="text-base text-ink-muted">{caption}</p>
			{/if}
		</div>
		{@render children()}
	</div>
</main>
