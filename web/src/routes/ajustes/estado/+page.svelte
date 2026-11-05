<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '../../../lib/api/app';
	import type { ApiError } from '../../../lib/api/errors';
	import type { Status } from '../../../lib/api/schemas';
	import { cache } from '../../../lib/cache/store.svelte';
	import LoadFailure from '../../../lib/components/LoadFailure.svelte';
	import PushActivate from '../../../lib/components/PushActivate.svelte';
	import StatusRow from '../../../lib/components/StatusRow.svelte';
	import { t } from '../../../lib/i18n';
	import { systemNow } from '../../../lib/time';
	import { statusLines } from '../../../lib/status';

	// Ajustes → Estado del sistema (design doc "Status screen"): what `GET /status` reports, with the
	// red lines marked, and "Activar notificaciones" while push needs her tap. The report is read
	// when the screen opens; it is not cached, so offline it says why it cannot load.
	let status = $state<Status | null>(null);
	let failure = $state<ApiError | null>(null);

	async function load(): Promise<void> {
		failure = null;
		const result = await api.getStatus();
		if (result.ok) {
			status = result.data;
		} else {
			failure = result.error;
		}
	}

	onMount(() => {
		void load();
	});

	const lines = $derived(
		status === null || cache.snapshot === null
			? []
			: statusLines(status, cache.snapshot.settings.timezone, systemNow())
	);
</script>

<div class="flex max-w-190 flex-col gap-4">
	<header class="flex min-h-12 items-center">
		<h1 class="text-3xl font-extrabold text-ink">{t('status_title')}</h1>
	</header>

	{#if failure !== null}
		<LoadFailure error={failure} onretry={() => void load()} />
	{:else if lines.length === 0}
		<p role="status" class="px-2 text-base text-ink-muted">{t('status_loading')}</p>
	{:else}
		<ul class="glass-card flex flex-col rounded-lg px-4 py-2">
			{#each lines as line, i (line.label)}
				{#if i > 0}
					<li class="h-px bg-glass-edge" aria-hidden="true"></li>
				{/if}
				<li><StatusRow label={t(line.label)} value={line.value} red={line.red} /></li>
			{/each}
		</ul>
	{/if}
	<PushActivate />
</div>
