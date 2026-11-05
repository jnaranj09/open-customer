<script lang="ts">
	import Card from '../../../lib/components/ui/Card.svelte';
	import { resolve } from '$app/paths';
	import { getContext } from 'svelte';
	import type { Rule } from '../../../lib/api/schemas';
	import { cache } from '../../../lib/cache/store.svelte';
	import EmptyState from '../../../lib/components/ui/EmptyState.svelte';
	import RuleSentence from '../../../lib/components/RuleSentence.svelte';
	import RuleSheet from '../../../lib/components/RuleSheet.svelte';
	import { t, type MessageKey } from '../../../lib/i18n';
	import { sortRules } from '../../../lib/rules';
	import type { createToaster } from '../../../lib/toaster.svelte';
	import WriteButton from '../../../lib/components/WriteButton.svelte';

	// Ajustes → Recordatorios (design doc item 77, CUJ 2 steps 3 and 4): the rules as sentences, with
	// a sheet to add one or edit the one she taps. Archived rules are listed apart, so she can
	// restore one. With no template yet there is nothing to send, so the screen leads to Plantillas.
	const toaster = getContext<ReturnType<typeof createToaster>>('settings-toaster');

	const data = $derived(cache.snapshot);
	const active = $derived(sortRules(data?.rules.filter((r) => !r.archived) ?? []));
	const archived = $derived(sortRules(data?.rules.filter((r) => r.archived) ?? []));
	const hasTemplate = $derived(data?.templates.some((x) => !x.archived) ?? false);

	// The rule in the sheet; 'new' is the empty form.
	let editing = $state<Rule | 'new' | null>(null);

	function done(message: MessageKey): void {
		editing = null;
		toaster.say(message);
	}
</script>

<div class="flex max-w-190 flex-col gap-4">
	<header class="flex min-h-12 flex-wrap items-center justify-between gap-3">
		<h1 class="text-3xl font-extrabold text-ink">{t('rules_title')}</h1>
		{#if data !== null && hasTemplate}
			<div class="w-44 shrink-0">
				<WriteButton
					variant="strong"
					onclick={() => {
						editing = 'new';
					}}>{t('rules_new')}</WriteButton
				>
			</div>
		{/if}
	</header>

	{#if data === null}
		<p role="status" class="px-2 text-base text-ink-muted">{t('rules_loading')}</p>
	{:else}
		{#if !hasTemplate && data.rules.length === 0}
			<Card>
				<EmptyState
					title={t('rules_templates_title')}
					body={t('rules_templates_body')}
					actionLabel={t('rules_templates_action')}
					href={resolve('/ajustes/plantillas')}
				/>
			</Card>
		{:else if data.rules.length === 0}
			<Card>
				<EmptyState title={t('due_no_rules_title')} body={t('due_no_rules_body')} />
			</Card>
		{:else}
			{@render rows(active)}
			{#if archived.length > 0}
				<h2 class="px-2 text-sm font-bold text-ink-muted">{t('archived_heading_masculine')}</h2>
				{@render rows(archived)}
			{/if}
		{/if}
		{#if editing !== null}
			<RuleSheet
				{...editing === 'new' ? {} : { rule: editing }}
				templates={data.templates}
				services={data.services}
				onclose={() => {
					editing = null;
				}}
				ondone={done}
			/>
		{/if}
	{/if}
</div>

{#snippet rows(list: Rule[])}
	{#if data !== null && list.length > 0}
		<Card as="ul" pad="list" class="flex flex-col gap-1">
			{#each list as rule (rule.id)}
				<li>
					<RuleSentence
						{rule}
						templates={data.templates}
						services={data.services}
						onclick={() => {
							editing = rule;
						}}
					/>
				</li>
			{/each}
		</Card>
	{/if}
{/snippet}
