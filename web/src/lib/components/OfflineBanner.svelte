<script lang="ts">
	import { t } from '../i18n';
	import { formatTime } from '../time';
	import Icon from './icons/Icon.svelte';

	let { lastSyncAt, zone }: { lastSyncAt: string | null; zone: string } = $props();

	const message = $derived(
		lastSyncAt === null
			? t('offline_banner_no_data')
			: t('offline_banner', { time: formatTime(lastSyncAt, zone) })
	);
</script>

<div
	role="status"
	class="glass-card flex items-center gap-3 rounded-lg bg-glass-tint px-5 py-3.5 text-ink"
>
	<Icon name="cloud-off" class="size-5.5 shrink-0 text-accent-700" />
	<p class="text-base">{message}</p>
</div>
