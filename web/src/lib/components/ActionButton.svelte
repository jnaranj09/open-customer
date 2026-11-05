<script lang="ts">
	import type { Snippet } from 'svelte';
	import { cache } from '../cache/store.svelte';
	import { t } from '../i18n';
	import Button from './Button.svelte';
	import type { ButtonVariant } from './buttonStyle';

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
	<Button
		{variant}
		{busy}
		{...cache.offline ? { disabled: true as const, disabledReason: t('offline_cannot_save') } : {}}
		{onclick}
	>
		{@render children()}
	</Button>
</div>
