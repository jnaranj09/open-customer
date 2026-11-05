<script lang="ts">
	// One line of a list on "Para enviar": a due item on the wide layout (a button that selects it),
	// or a sent one under "Enviados" (plain, with the time).
	let {
		title,
		subtitle,
		meta,
		selected = false,
		onselect
	}: {
		title: string;
		subtitle?: string | null | undefined;
		meta?: string;
		selected?: boolean;
		onselect?: () => void;
	} = $props();

	const ROW =
		'glass-card flex min-h-tap w-full items-center gap-3 rounded-lg px-5 py-3 text-left text-ink';
</script>

{#snippet content()}
	<span class="flex min-w-0 flex-1 flex-col gap-0.5">
		<span class="truncate text-base font-bold">{title}</span>
		{#if subtitle}
			<span class="line-clamp-2 text-sm text-ink-muted">{subtitle}</span>
		{/if}
	</span>
	{#if meta !== undefined}
		<span class="shrink-0 text-sm text-ink-muted">{meta}</span>
	{/if}
{/snippet}

{#if onselect === undefined}
	<div class="{ROW} {selected ? 'outline-2 outline-accent' : ''}">
		{@render content()}
	</div>
{:else}
	<button
		type="button"
		onclick={onselect}
		aria-current={selected ? 'true' : undefined}
		class="{ROW} hover:bg-accent-100 {selected ? 'outline-2 outline-accent' : ''}"
	>
		{@render content()}
	</button>
{/if}
