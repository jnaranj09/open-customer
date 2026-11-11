<script lang="ts">
	import { fieldClasses } from './fieldStyle';
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
		hint,
		lines
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
		/** A text of several lines: a `<textarea>` this many lines tall (its `type` is ignored). */
		lines?: number;
	} = $props();

	const id = $props.id();
	const noteId = `${id}-note`;
	const note = $derived(error ?? hint);
</script>

<div class="flex flex-col gap-1.5">
	<label for={id} class="text-sm font-bold text-ink">{label}</label>
	{#if lines === undefined}
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
			class="{fieldClasses(
				error !== undefined
			)} h-12 w-full px-4 text-ink placeholder:text-ink-muted"
		/>
	{:else}
		<textarea
			{id}
			rows={lines}
			{autocomplete}
			{autocapitalize}
			{placeholder}
			bind:value
			aria-invalid={error !== undefined}
			aria-describedby={note === undefined ? undefined : noteId}
			class="{fieldClasses(
				error !== undefined
			)} w-full px-4 py-3 text-ink placeholder:text-ink-muted"></textarea>
	{/if}
	{#if note !== undefined}
		<p id={noteId} class="text-sm {error === undefined ? 'text-ink-muted' : 'text-error'}">
			{note}
		</p>
	{/if}
</div>
