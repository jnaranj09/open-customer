<script lang="ts">
	import type { Snippet } from 'svelte';
	import { cache } from '../cache/store.svelte';
	import { t } from '../i18n';
	import Button from './ui/Button.svelte';
	import { OFFLINE_REASON_ID } from './ui/offlineReason';
	import type { ButtonVariant } from './ui/buttonStyle';

	// A button that writes to the server (save, archive, confirm). The server holds the truth and
	// nothing is queued, so offline it is off and says why (design doc "Phone cache and offline
	// behavior"): under itself, or, with `sharedOfflineReason`, through the offline banner's own
	// line (a list of cards, where the reason under each card would repeat).
	let {
		variant,
		busy = false,
		type = 'button',
		sharedOfflineReason = false,
		onclick,
		children
	}: {
		variant: ButtonVariant;
		busy?: boolean;
		type?: 'button' | 'submit';
		sharedOfflineReason?: boolean;
		onclick?: () => void;
		children: Snippet;
	} = $props();

	const offline = $derived(
		!cache.offline
			? {}
			: sharedOfflineReason
				? { disabled: true as const, reasonId: OFFLINE_REASON_ID }
				: { disabled: true as const, disabledReason: t('offline_cannot_save') }
	);
</script>

<Button {variant} {busy} {type} {...onclick === undefined ? {} : { onclick }} {...offline}>
	{@render children()}
</Button>
