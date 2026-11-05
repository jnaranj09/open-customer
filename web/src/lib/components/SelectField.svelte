<script lang="ts">
	import { t } from '../i18n';
	import type { Option } from '../setup';

	let {
		label,
		value = $bindable(),
		options,
		error
	}: {
		label: string;
		value: string;
		options: readonly Option[];
		error?: string | undefined;
	} = $props();

	const id = $props.id();
	const noteId = `${id}-note`;
</script>

<div class="flex flex-col gap-1.5">
	<label for={id} class="text-sm font-bold text-ink">{label}</label>
	<select
		{id}
		bind:value
		aria-invalid={error !== undefined}
		aria-describedby={error === undefined ? undefined : noteId}
		class="h-12 w-full rounded-sm border bg-glass-strong px-4 text-base text-ink focus:border-accent {error ===
		undefined
			? 'border-glass-edge'
			: 'border-error'}"
	>
		<option value="" disabled>{t('select_placeholder')}</option>
		{#each options as option (option.id)}
			<option value={option.id}>{option.label}</option>
		{/each}
	</select>
	{#if error !== undefined}
		<p id={noteId} class="text-sm text-error">{error}</p>
	{/if}
</div>
