<script lang="ts">
	import type { Snippet } from 'svelte';
	import Button from './ui/Button.svelte';
	import type { ButtonVariant } from './ui/buttonStyle';

	// An app button that opens the file chooser. The browser's own file box shows "Choose File" in
	// the phone's language and makes that its accessible name; here the button carries the Spanish
	// text and the real input stays hidden (out of the tab order and the accessibility tree: the
	// button is the control). The picked file goes to `onpick`; the input is cleared, so picking
	// the same file again fires again.
	let {
		variant,
		accept,
		busy = false,
		disabledReason,
		onpick,
		children
	}: {
		variant: ButtonVariant;
		accept: string;
		busy?: boolean;
		/** Given, the button is disabled and says why. */
		disabledReason?: string | undefined;
		onpick: (file: File) => void;
		children: Snippet;
	} = $props();

	let input: HTMLInputElement | undefined = $state();
</script>

<input
	bind:this={input}
	type="file"
	{accept}
	tabindex="-1"
	aria-hidden="true"
	class="sr-only"
	onchange={(e) => {
		const file = e.currentTarget.files?.[0];
		e.currentTarget.value = '';
		if (file !== undefined) {
			onpick(file);
		}
	}}
/>
<Button
	{variant}
	{busy}
	{...disabledReason === undefined ? {} : { disabled: true as const, disabledReason }}
	onclick={() => {
		input?.click();
	}}
>
	{@render children()}
</Button>
