<script lang="ts">
	import Card from './ui/Card.svelte';

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
	<Card
		pad="message"
		{selected}
		class="flex min-h-tap w-full items-center gap-3 text-left text-ink"
	>
		{@render content()}
	</Card>
{:else}
	<Card
		as="button"
		type="button"
		onclick={onselect}
		aria-current={selected ? 'true' : undefined}
		pad="message"
		{selected}
		class="flex min-h-tap w-full items-center gap-3 text-left text-ink hover:bg-accent-100"
	>
		{@render content()}
	</Card>
{/if}
