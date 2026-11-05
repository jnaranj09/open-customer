<script lang="ts">
	import { iconVersion } from '../iconVersion';

	// The business icon. Without an uploaded icon it draws the built-in one (a light disc on the
	// accent, the shape the server draws for the home screen) in CSS, so it follows `--accent` as
	// soon as the color changes. `custom` undefined (no cached settings yet) loads the served PNG,
	// which the server also draws in the accent.
	let { custom, class: shape }: { custom: boolean | undefined; class: string } = $props();

	// The icon is never cached (the manifest rules): offline it cannot load, so a failed load
	// falls back to the drawn icon instead of a broken image box.
	let failed = $state(false);
</script>

{#if custom !== false && !failed}
	<img
		src="/icons/512.png?v={$iconVersion}"
		alt=""
		class="{shape} object-cover"
		onerror={() => {
			failed = true;
		}}
	/>
{:else}
	<div aria-hidden="true" class="{shape} grid place-items-center bg-accent">
		<div class="size-3/5 rounded-pill bg-bg-100"></div>
	</div>
{/if}
