<script lang="ts">
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { setContext, type Snippet } from 'svelte';
	import type { Template } from '../../../lib/api/schemas';
	import { cache } from '../../../lib/cache/store.svelte';
	import EmptyState from '../../../lib/components/EmptyState.svelte';
	import ListRow from '../../../lib/components/ListRow.svelte';
	import Toast from '../../../lib/components/Toast.svelte';
	import { t } from '../../../lib/i18n';
	import { createToaster } from '../../../lib/toaster.svelte';
	import WriteLink from '../../../lib/components/WriteLink.svelte';

	// Ajustes → Plantillas (design doc item 77): the list of message templates, with the open
	// template or the new-template form beside it. The two panes follow the width this page is
	// given (a container query), not the window: with room they sit side by side; narrower, the
	// list is one screen and the editor the next. The page has no shell of its own: the Ajustes
	// layout frames it.
	let { children }: { children: Snippet } = $props();

	const toaster = createToaster();
	setContext('templates-toaster', toaster);

	const templates = $derived(cache.snapshot?.templates ?? []);
	const active = $derived(templates.filter((x) => !x.archived));
	const archived = $derived(templates.filter((x) => x.archived));
	const onList = $derived(page.url.pathname.replace(/\/$/, '') === resolve('/ajustes/plantillas'));
</script>

{#snippet rows(list: Template[])}
	<ul class="glass-card flex flex-col gap-1 rounded-lg p-2">
		{#each list as template (template.id)}
			<li>
				<ListRow
					href={resolve('/ajustes/plantillas/[id]', { id: template.id })}
					title={template.name}
					selected={template.id === page.params['id']}
				/>
			</li>
		{/each}
	</ul>
{/snippet}

<div class="@container">
	<div class="flex items-start gap-8">
		<div
			class="flex min-w-0 flex-col gap-4 @3xl:w-80 @3xl:shrink-0 {onList
				? 'w-full'
				: 'hidden @3xl:flex'}"
		>
			<header class="flex min-h-12 items-center justify-between gap-3">
				<h1 class="text-3xl font-extrabold text-ink">{t('templates_title')}</h1>
				<WriteLink href={resolve('/ajustes/plantillas/nueva')} variant="primary" width="w-44">
					{t('templates_new')}
				</WriteLink>
			</header>

			{#if cache.snapshot === null}
				<p role="status" class="px-2 text-base text-ink-muted">{t('templates_loading')}</p>
			{:else if templates.length === 0}
				<div class="glass-card rounded-lg">
					<EmptyState title={t('templates_empty_title')} body={t('templates_empty_body')} />
				</div>
			{:else}
				{@render rows(active)}
				{#if archived.length > 0}
					<h2 class="px-2 text-sm font-bold text-ink-muted">{t('templates_archived_heading')}</h2>
					{@render rows(archived)}
				{/if}
			{/if}
		</div>
		<div class="min-w-0 flex-1 {onList ? 'hidden @3xl:block' : ''}">
			{@render children()}
		</div>
	</div>
</div>

{#if toaster.current !== null}
	{#key toaster.current.n}
		<Toast message={toaster.current.message} />
	{/key}
{/if}
