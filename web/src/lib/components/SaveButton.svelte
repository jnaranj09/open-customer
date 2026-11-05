<script lang="ts">
	import type { Snippet } from 'svelte';
	import { cache } from '../cache/store.svelte';
	import { t } from '../i18n';
	import Button from './Button.svelte';

	// The submit button of a form that saves to the server. Offline it is off and says why.
	let { busy, children }: { busy: boolean; children: Snippet } = $props();
</script>

<Button
	variant="primary"
	type="submit"
	{busy}
	{...cache.offline ? { disabled: true as const, disabledReason: t('offline_cannot_save') } : {}}
>
	{@render children()}
</Button>
