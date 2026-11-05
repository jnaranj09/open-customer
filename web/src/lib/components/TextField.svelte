<script lang="ts">
	import type { HTMLInputAttributes } from 'svelte/elements';

	let {
		label,
		value = $bindable(),
		type = 'text',
		error,
		autocomplete,
		autocapitalize,
		autocorrect,
		inputmode,
		placeholder,
		hint
	}: {
		label: string;
		value: string;
		type?: 'text' | 'password' | 'tel' | 'email' | 'search' | 'date' | 'time';
		error?: string | undefined;
		autocomplete?: HTMLInputAttributes['autocomplete'];
		autocapitalize?: HTMLInputAttributes['autocapitalize'];
		autocorrect?: HTMLInputAttributes['autocorrect'];
		inputmode?: HTMLInputAttributes['inputmode'];
		placeholder?: string;
		hint?: string;
	} = $props();

	const id = $props.id();
	const noteId = `${id}-note`;
	const note = $derived(error ?? hint);
</script>

<div class="flex flex-col gap-1.5">
	<label for={id} class="text-sm font-bold text-ink">{label}</label>
	<input
		{id}
		{type}
		{autocomplete}
		{autocapitalize}
		{autocorrect}
		{inputmode}
		{placeholder}
		bind:value
		aria-invalid={error !== undefined}
		aria-describedby={note === undefined ? undefined : noteId}
		class="h-12 w-full rounded-sm border bg-glass-strong px-4 text-base text-ink placeholder:text-ink-muted focus:border-accent {error ===
		undefined
			? 'border-glass-edge'
			: 'border-error'}"
	/>
	{#if note !== undefined}
		<p id={noteId} class="text-sm {error === undefined ? 'text-ink-muted' : 'text-error'}">
			{note}
		</p>
	{/if}
</div>
