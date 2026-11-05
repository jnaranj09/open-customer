<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { ButtonVariant } from './ui/buttonStyle';
	import WriteButton from './WriteButton.svelte';

	// One of the buttons in a detail card's action row. It changes data on the server, so offline
	// it is off and says why.
	let {
		variant,
		busy = false,
		onclick,
		children
	}: { variant: ButtonVariant; busy?: boolean; onclick: () => void; children: Snippet } = $props();
</script>

<div class="min-w-36 flex-1">
	<WriteButton {variant} {busy} {onclick}>
		{@render children()}
	</WriteButton>
</div>
