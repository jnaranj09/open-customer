<script lang="ts">
	import Card from './ui/Card.svelte';
	import { resolve } from '$app/paths';
	import type { DueItem } from '../api/schemas';
	import { t } from '../i18n';
	import { layout } from '../layout';
	import { buttonClasses } from './ui/buttonStyle';
	import Icon from './icons/Icon.svelte';
	import NoRulesState from './NoRulesState.svelte';

	let { items, hasRules }: { items: DueItem[]; hasRules: boolean } = $props();

	// The wide rail lists this many; the rest are one tap away on "Para enviar".
	const SHOWN = 5;

	const summary = $derived(
		items.length === 0
			? t('due_today_none')
			: items.length === 1
				? t('due_today_summary_one')
				: t('due_today_summary_other', { count: items.length })
	);
</script>

{#if !hasRules}
	<NoRulesState />
{:else if layout.current === 'wide'}
	<Card as="section" pad="md" class="flex flex-col gap-3">
		<h2 class="text-xl font-bold text-ink">{t('due_today_heading')}</h2>
		<a
			href={resolve('/enviar')}
			class="flex min-h-tap items-center gap-2 text-base font-bold text-accent-text hover:underline"
		>
			{summary}
			<Icon name="chevron-right" class="size-4.5 text-ink-muted" />
		</a>
		{#if items.length > 0}
			<ul class="flex flex-col gap-2">
				{#each items.slice(0, SHOWN) as item (item.id)}
					<li class="flex items-center gap-3">
						<p class="min-w-0 flex-1 truncate text-base text-ink">{item.client_name}</p>
						<a
							href={resolve(`/enviar?item=${item.id}` as const)}
							class="{buttonClasses('ghost')} shrink-0"
						>
							{t('due_send')}
						</a>
					</li>
				{/each}
			</ul>
		{/if}
	</Card>
{:else}
	<Card
		as="a"
		href={resolve('/enviar')}
		variant="tint"
		pad="md"
		class="flex items-center gap-4 text-ink hover:bg-accent-100"
	>
		<Icon name="send" class="size-7 shrink-0 text-accent-text" />
		<span class="flex min-w-0 flex-1 flex-col gap-1">
			<span class="text-xl font-extrabold">{summary}</span>
			{#if items.length > 0}
				<span class="text-sm text-ink-muted">{t('due_today_hint')}</span>
			{/if}
		</span>
		<Icon name="chevron-right" class="size-4.5 shrink-0 text-ink-muted" />
	</Card>
{/if}
