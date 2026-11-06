<script lang="ts">
	import { resolve } from '$app/paths';
	import type { Service } from '../api/schemas';
	import { removeService, tapService, type Line } from '../booking';
	import { t } from '../i18n';
	import Chip from './ui/Chip.svelte';
	import EmptyState from './ui/EmptyState.svelte';
	import IconButton from './ui/IconButton.svelte';

	// The services chips (CUJ 3): a tap picks a service, a second tap adds a copy (shown "×2"), and
	// the minus beside a picked chip takes one copy away. A service archived since the booking shows
	// only while it is on the appointment.
	let { services, lines = $bindable() }: { services: Service[]; lines: Line[] } = $props();

	const shown = $derived(
		services.filter((s) => !s.archived || lines.some((l) => l.service_id === s.id))
	);
	const labelId = $props.id();
	const countOf = (id: string): number => lines.find((l) => l.service_id === id)?.count ?? 0;
</script>

<div class="flex flex-col gap-1.5">
	<span id={labelId} class="text-sm font-bold text-ink">{t('appointment_services')}</span>
	{#if shown.length === 0}
		<EmptyState
			title={t('appointment_services_empty_title')}
			body={t('appointment_services_empty_body')}
			actionLabel={t('appointment_services_empty_action')}
			href={resolve('/ajustes/servicios')}
		/>
	{:else}
		<div role="group" aria-labelledby={labelId} class="flex flex-wrap gap-2">
			{#each shown as service (service.id)}
				{@const count = countOf(service.id)}
				<Chip
					variant={count > 0 ? 'selected' : 'default'}
					onclick={() => {
						lines = tapService(lines, service.id);
					}}
				>
					{service.name}
					{#if count > 1}
						<span class="text-accent-text">{t('appointment_service_count', { count })}</span>
					{/if}
					{#snippet trailing()}
						{#if count > 0}
							<IconButton
								variant="accent"
								icon="minus"
								label={t('appointment_service_remove', { name: service.name })}
								onclick={() => {
									lines = removeService(lines, service.id);
								}}
							/>
						{/if}
					{/snippet}
				</Chip>
			{/each}
		</div>
	{/if}
</div>
