<script lang="ts">
	import { getContext } from 'svelte';
	import type { Attribute } from '../../../lib/api/schemas';
	import { inScope, SCOPE_LABELS, TYPE_LABELS } from '../../../lib/attributes';
	import { cache } from '../../../lib/cache/store.svelte';
	import AttributeSheet from '../../../lib/components/AttributeSheet.svelte';
	import Button from '../../../lib/components/Button.svelte';
	import EmptyState from '../../../lib/components/EmptyState.svelte';
	import ListRow from '../../../lib/components/ListRow.svelte';
	import { t, type MessageKey } from '../../../lib/i18n';
	import type { createToaster } from '../../../lib/toaster.svelte';

	// Ajustes → Atributos (design doc item 77, CUJ 2 step 2): the attributes grouped by what they
	// belong to (the client or each appointment), and a sheet to add one or edit the one she taps.
	// The sheet follows the attribute by id, so the options it shows come from the refreshed cache.
	const toaster = getContext<ReturnType<typeof createToaster>>('settings-toaster');

	const data = $derived(cache.snapshot);
	const scopes = ['client', 'appointment'] as const;

	// What the sheet shows: a new attribute, the one with this id, or nothing.
	let creating = $state(false);
	let editingId = $state<string | null>(null);
	const open = $derived(data?.attributes.find((a) => a.id === editingId));

	function done(message: MessageKey): void {
		editingId = null;
		toaster.say(message);
	}
</script>

{#snippet rows(list: Attribute[])}
	<ul class="glass-card flex flex-col gap-1 rounded-lg p-2">
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
	</ul>
{/snippet}

<div class="flex max-w-190 flex-col gap-4">
	<header class="flex min-h-12 items-center justify-between gap-3">
		<h1 class="font-display text-3xl font-semibold text-ink">{t('attributes_title')}</h1>
		{#if data !== null}
			<div class="w-44 shrink-0">
				<Button
					variant="primary"
					onclick={() => {
						creating = true;
					}}>{t('attributes_new')}</Button
				>
			</div>
		{/if}
	</header>

	{#if data === null}
		<p role="status" class="px-2 text-base text-ink-muted">{t('attributes_loading')}</p>
	{:else}
		{#if data.attributes.length === 0}
			<div class="glass-card rounded-lg">
				<EmptyState title={t('attributes_empty_title')} body={t('attributes_empty_body')} />
			</div>
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
