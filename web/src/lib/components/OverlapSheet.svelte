<script lang="ts">
	import type { Appointment } from '../api/schemas';
	import { t } from '../i18n';
	import { formatTime } from '../time';
	import BottomSheet from './BottomSheet.svelte';
	import Button from './Button.svelte';
	import WriteButton from './WriteButton.svelte';

	// The 409 `overlap` answer (CUJ 3): the appointments in the way, and "Agendar igual" to save
	// anyway (the caller sends the same change again with `allow_overlap`).
	let {
		open,
		overlapping,
		zone,
		busy,
		onconfirm,
		onclose
	}: {
		open: boolean;
		overlapping: Appointment[];
		zone: string;
		busy: boolean;
		onconfirm: () => void;
		onclose: () => void;
	} = $props();
</script>

<BottomSheet {open} title={t('overlap_title')} {onclose}>
	<ul class="flex flex-col gap-2">
		{#each overlapping as appt (appt.id)}
			<li class="text-base text-ink">
				<span class="font-bold">{appt.client_name}</span>
				{formatTime(appt.starts_at, zone)} – {formatTime(appt.ends_at, zone)}
			</li>
		{/each}
	</ul>
	<WriteButton variant="primary" {busy} onclick={onconfirm}>{t('overlap_confirm')}</WriteButton>
	<Button variant="secondary" onclick={onclose}>{t('cancel')}</Button>
</BottomSheet>
