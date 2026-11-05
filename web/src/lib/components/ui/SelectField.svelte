<script lang="ts">
	import { fieldClasses } from './fieldStyle';
	import { t } from '../../i18n';
	import type { Option } from '../../setup';

	let {
		label,
		value = $bindable(),
		options,
		clearLabel,
		error
	}: {
		label: string;
		value: string;
		options: readonly Option[];
		/** Given, the empty choice can be picked again (it clears the field) and reads this text. */
		clearLabel?: string;
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
		class="{fieldClasses(error !== undefined)} h-12 w-full px-4 text-ink"
	>
		<option value="" disabled={clearLabel === undefined}>
			{clearLabel ?? t('select_placeholder')}
		</option>
		{#each options as option (option.id)}
			<option value={option.id}>{option.label}</option>
		{/each}
	</select>
	{#if error !== undefined}
		<p id={noteId} class="text-sm text-error">{error}</p>
	{/if}
</div>
