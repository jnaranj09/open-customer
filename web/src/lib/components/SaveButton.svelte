<script lang="ts">
	import type { Snippet } from 'svelte';
	import WriteButton from './WriteButton.svelte';

	// The submit button of a form that saves to the server. Offline it is off and says why.
	let { busy, children }: { busy: boolean; children: Snippet } = $props();
</script>

<WriteButton variant="primary" type="submit" {busy}>
	{@render children()}
</WriteButton>
