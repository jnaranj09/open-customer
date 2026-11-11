<script lang="ts">
	import { onDestroy } from 'svelte';
	import type { QuickReply } from '../../../lib/api/schemas';
	import { cache } from '../../../lib/cache/store.svelte';
	import { copiedFlag, copyText } from '../../../lib/copy';
	import Icon from '../../../lib/components/icons/Icon.svelte';
	import ReplySheet from '../../../lib/components/ReplySheet.svelte';
	import Button from '../../../lib/components/ui/Button.svelte';
	import Callout from '../../../lib/components/ui/Callout.svelte';
	import Card from '../../../lib/components/ui/Card.svelte';
	import EmptyState from '../../../lib/components/ui/EmptyState.svelte';
	import ListRow from '../../../lib/components/ui/ListRow.svelte';
	import TextField from '../../../lib/components/ui/TextField.svelte';
	import WriteButton from '../../../lib/components/WriteButton.svelte';
	import { t, type MessageKey } from '../../../lib/i18n';
	import { filterReplies } from '../../../lib/replies';
	import { toaster } from '../../../lib/toaster.svelte';

	// Enviar → Respuestas (reminders-and-replies design, "Quick replies"): her own messages in her
	// order, a search over the title and the text, "Copiar" on each, and a sheet to add, edit, move,
	// archive or restore one. The list is the cached copy, so search and copy work offline.

	const data = $derived(cache.snapshot);
	const all = $derived(data?.quick_replies ?? []);
	const active = $derived(all.filter((r) => !r.archived));
	const archived = $derived(all.filter((r) => r.archived));

	let query = $state('');
	let showArchived = $state(false);
	const shown = $derived(filterReplies(active, query));
	const shownArchived = $derived(filterReplies(archived, query));

	// The reply in the sheet; 'new' is the empty form.
	let editing = $state<QuickReply | 'new' | null>(null);
	// The reply whose "Copiar" just worked, and a failed copy.
	let copiedId = $state<string | null>(null);
	let copyFailed = $state(false);
	let lastCopied: string | null = null;
	const copiedNote = copiedFlag((on) => {
		copiedId = on ? lastCopied : null;
	});

	function copy(reply: QuickReply): void {
		copyFailed = false;
		copyText(
			reply.body,
			() => {
				lastCopied = reply.id;
				copiedNote.show();
			},
			() => {
				copyFailed = true;
			}
		);
	}

	function done(message: MessageKey): void {
		editing = null;
		toaster.say(message);
	}

	onDestroy(() => {
		copiedNote.stop();
	});
</script>

{#snippet rows(list: QuickReply[])}
	<Card as="ul" pad="list" class="flex flex-col">
		{#each list as reply (reply.id)}
			<li class="flex items-center gap-2">
				<div class="min-w-0 flex-1">
					<ListRow
						title={reply.title}
						subtitle={reply.body}
						onclick={() => {
							editing = reply;
						}}
					/>
				</div>
				<div class="w-32 shrink-0">
					<Button
						variant="glass"
						onclick={() => {
							copy(reply);
						}}
					>
						{#if copiedId === reply.id}
							<Icon name="check" class="size-4.5" />
							{t('quote_copied')}
						{:else}
							{t('quote_copy')}
						{/if}
					</Button>
				</div>
			</li>
		{/each}
	</Card>
{/snippet}

<!-- A message for screen readers only: the copy needs no toast. -->
<span class="sr-only" aria-live="polite">{copiedId === null ? '' : t('quote_copied')}</span>

<div class="flex max-w-190 flex-col gap-4">
	{#if data === null}
		<p role="status" class="px-2 text-base text-ink-muted">{t('replies_loading')}</p>
	{:else}
		<div class="w-44">
			<WriteButton
				variant="strong"
				onclick={() => {
					editing = 'new';
				}}>{t('replies_new')}</WriteButton
			>
		</div>
		{#if all.length === 0}
			<Card>
				<EmptyState title={t('replies_empty_title')} body={t('replies_empty_body')} />
			</Card>
		{:else}
			<TextField label={t('replies_search')} type="search" bind:value={query} autocomplete="off" />
			{#if copyFailed}
				<Callout role="alert" class="text-sm">{t('quote_copy_failed')}</Callout>
			{/if}
			{#if shown.length > 0}
				{@render rows(shown)}
			{:else if active.length > 0}
				<p class="px-2 text-base text-ink-muted">{t('replies_no_results')}</p>
			{/if}
			{#if archived.length > 0}
				<button
					type="button"
					class="flex min-h-tap items-center gap-2 self-start px-4 text-sm font-bold text-ink-muted"
					aria-expanded={showArchived}
					onclick={() => {
						showArchived = !showArchived;
					}}
				>
					<Icon name={showArchived ? 'chevron-down' : 'chevron-right'} class="size-4.5" />
					{t('replies_archived_toggle', { count: archived.length })}
				</button>
				{#if showArchived && shownArchived.length > 0}
					{@render rows(shownArchived)}
				{/if}
			{/if}
		{/if}
	{/if}
</div>

{#if editing !== null}
	<ReplySheet
		{...editing === 'new' ? {} : { reply: editing }}
		{active}
		onclose={() => {
			editing = null;
		}}
		ondone={done}
	/>
{/if}
