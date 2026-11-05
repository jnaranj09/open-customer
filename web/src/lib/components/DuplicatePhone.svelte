<script lang="ts">
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

<p role="alert" class="rounded-md bg-glass-tint px-4 py-3 text-sm font-bold text-ink">
	{t('client_phone_taken', { names: names.join(', ') })}
</p>
<WriteButton variant="primary" {busy} onclick={onconfirm}>{label}</WriteButton>
