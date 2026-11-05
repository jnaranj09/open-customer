<script lang="ts">
	import Callout from './ui/Callout.svelte';
	import { t } from '../i18n';
	import WriteButton from './WriteButton.svelte';

	// The warning for a phone another client has (design doc "Clients and search"), with the button
	// that goes ahead anyway. The caller sends `allow_duplicate_phone` when it is tapped.
	let {
		names,
		busy,
		label,
		onconfirm
	}: { names: string[]; busy: boolean; label: string; onconfirm: () => void } = $props();
</script>

<Callout as="p" role="alert" class="text-sm font-bold">
	{t('client_phone_taken', { names: names.join(', ') })}
</Callout>
<WriteButton variant="strong" {busy} onclick={onconfirm}>{label}</WriteButton>
