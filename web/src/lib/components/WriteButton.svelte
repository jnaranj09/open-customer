<script lang="ts">
	import type { Snippet } from 'svelte';
	import { cache } from '../cache/store.svelte';
	import { t } from '../i18n';
	import Button from './Button.svelte';
	import type { ButtonVariant } from './buttonStyle';

	// A button that writes to the server (save, archive, confirm). The server holds the truth and
	// nothing is queued, so offline it is off and says why (design doc "Phone cache and offline
	// behavior").
	let {
		variant,
		busy = false,
		type = 'button',
		onclick,
		children
	}: {
		variant: ButtonVariant;
		busy?: boolean;
		type?: 'button' | 'submit';
		onclick?: () => void;
		children: Snippet;
	} = $props();
</script>

<Button
	{variant}
	{busy}
	{type}
	{...onclick === undefined ? {} : { onclick }}
	{...cache.offline ? { disabled: true as const, disabledReason: t('offline_cannot_save') } : {}}
>
	{@render children()}
</Button>
