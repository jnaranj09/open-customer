<script lang="ts">
	import { t } from '../../i18n';
	import { formatTime } from '../../time';
	import Icon from '../icons/Icon.svelte';

	let { lastSyncAt, zone }: { lastSyncAt: string | null; zone: string } = $props();

	const message = $derived(
		lastSyncAt === null
			? t('offline_banner_no_data')
			: t('offline_banner', { time: formatTime(lastSyncAt, zone) })
	);
</script>

<!-- The one place that says why nothing can be saved: the disabled controls stay put, faded or
     with their own short reason, and no note floats over the screen. -->
<div
	role="status"
	class="glass-card flex items-center gap-3 rounded-lg bg-glass-tint px-5 py-3.5 text-ink"
>
	<Icon name="cloud-off" class="size-5.5 shrink-0 text-accent-text" />
	<div class="flex min-w-0 flex-col gap-0.5">
		<p class="text-base">{message}</p>
		<p class="text-sm text-ink-muted">{t('offline_cannot_save')}</p>
	</div>
</div>
