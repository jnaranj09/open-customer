<script lang="ts">
	import Card from '../../../lib/components/ui/Card.svelte';
	import type { Attribute } from '../../../lib/api/schemas';
	import { inScope, SCOPE_LABELS, TYPE_LABELS } from '../../../lib/attributes';
	import { cache } from '../../../lib/cache/store.svelte';
	import AttributeSheet from '../../../lib/components/AttributeSheet.svelte';
	import EmptyState from '../../../lib/components/ui/EmptyState.svelte';
	import ListRow from '../../../lib/components/ui/ListRow.svelte';
	import { t, type MessageKey } from '../../../lib/i18n';
	import { toaster } from '../../../lib/toaster.svelte';
	import WriteButton from '../../../lib/components/WriteButton.svelte';

	// Ajustes → Atributos (design doc item 77, CUJ 2 step 2): the attributes grouped by what they
	// belong to (the client or each appointment), and a sheet to add one or edit the one she taps.
	// The sheet follows the attribute by id, so the options it shows come from the refreshed cache.

	const data = $derived(cache.snapshot);
	const scopes = ['client', 'appointment'] as const;

	// What the sheet shows: a new attribute, the one with this id, or nothing.
	let creating = $state(false);
	let editingId = $state<string | null>(null);
	const open = $derived(data?.attributes.find((a) => a.id === editingId));

	// Closes whichever sheet is open: a new text, number or date attribute is done at creation.
	function done(message: MessageKey): void {
		creating = false;
		editingId = null;
		toaster.say(message);
	}
</script>

{#snippet rows(list: Attribute[])}
	<Card as="ul" pad="list" class="flex flex-col gap-1">
		{#each list as attribute (attribute.id)}
			<li>
				<ListRow
					title={attribute.label}
					subtitle={t(TYPE_LABELS[attribute.type])}
					onclick={() => {
						editingId = attribute.id;
					}}
				/>
			</li>
		{/each}
	</Card>
{/snippet}

<div class="flex max-w-190 flex-col gap-4">
	<header class="flex min-h-12 items-center justify-between gap-3">
		<h1 class="text-3xl font-extrabold text-ink">{t('attributes_title')}</h1>
		{#if data !== null}
			<div class="w-44 shrink-0">
				<WriteButton
					variant="strong"
					onclick={() => {
						creating = true;
					}}>{t('attributes_new')}</WriteButton
				>
			</div>
		{/if}
	</header>

	{#if data === null}
		<p role="status" class="px-2 text-base text-ink-muted">{t('attributes_loading')}</p>
	{:else}
		{#if data.attributes.length === 0}
			<Card>
				<EmptyState title={t('attributes_empty_title')} body={t('attributes_empty_body')} />
			</Card>
		{/if}
		{#each scopes as scope (scope)}
			{@const list = inScope(data.attributes, scope, false)}
			{#if list.length > 0}
				<h2 class="px-2 text-sm font-bold text-ink-muted">{t(SCOPE_LABELS[scope])}</h2>
				{@render rows(list)}
			{/if}
		{/each}
		{#each scopes as scope (scope)}
			{@const list = inScope(data.attributes, scope, true)}
			{#if list.length > 0}
				<h2 class="px-2 text-sm font-bold text-ink-muted">
					{t('archived_heading_masculine')} · {t(SCOPE_LABELS[scope])}
				</h2>
				{@render rows(list)}
			{/if}
		{/each}
		{#if creating}
			<AttributeSheet
				onclose={() => {
					creating = false;
				}}
				oncreated={(id: string) => {
					toaster.say('attribute_created');
					creating = false;
					editingId = id;
				}}
				ondone={done}
			/>
		{:else if open !== undefined}
			<AttributeSheet
				attribute={open}
				onclose={() => {
					editingId = null;
				}}
				oncreated={(id: string) => {
					editingId = id;
				}}
				ondone={done}
			/>
		{/if}
	{/if}
</div>
