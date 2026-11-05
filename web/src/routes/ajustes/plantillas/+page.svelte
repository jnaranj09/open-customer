<script lang="ts">
	import type { Template } from '../../../lib/api/schemas';
	import { cache } from '../../../lib/cache/store.svelte';
	import EmptyState from '../../../lib/components/EmptyState.svelte';
	import Icon from '../../../lib/components/icons/Icon.svelte';
	import ListRow from '../../../lib/components/ListRow.svelte';
	import TemplateForm from '../../../lib/components/TemplateForm.svelte';
	import Toast from '../../../lib/components/Toast.svelte';
	import { buttonClasses } from '../../../lib/components/buttonStyle';
	import { t } from '../../../lib/i18n';
	import { layout } from '../../../lib/layout';
	import { createToaster } from '../../../lib/toaster.svelte';

	// Ajustes → Plantillas (design doc item 77): the list of message templates and the editor. On
	// wide they sit side by side; on compact the list is one screen and the editor the next. Which
	// one is open is `open` ("nueva" or a template id). The page has no shell of its own: the
	// Ajustes layout frames it.
	const NEW = 'nueva';
	const toaster = createToaster();

	let open = $state<string | null>(null);
	const wide = $derived(layout.current === 'wide');
	const templates = $derived(cache.snapshot?.templates ?? []);
	const active = $derived(templates.filter((x) => !x.archived));
	const archived = $derived(templates.filter((x) => x.archived));
	const creating = $derived(open === NEW);
	const current = $derived(templates.find((x) => x.id === open));
	const editing = $derived(creating || current !== undefined);

	function show(next: string | null): void {
		open = next;
	}

	function saved(template: Template): void {
		toaster.say('template_saved');
		show(template.id);
	}
</script>

{#snippet rows(list: Template[])}
	<ul class="glass-card flex flex-col gap-1 rounded-lg p-2">
		{#each list as template (template.id)}
			<li>
				<ListRow
					title={template.name}
					selected={template.id === current?.id}
					onclick={() => {
						show(template.id);
					}}
				/>
			</li>
		{/each}
	</ul>
{/snippet}

<div class="flex items-start gap-8">
	<div
		class="flex min-w-0 flex-col gap-4 {wide ? 'w-80 shrink-0' : 'w-full'} {wide || !editing
			? ''
			: 'hidden'}"
	>
		<header class="flex items-center justify-between gap-3">
			<h1 class="font-display text-3xl font-semibold text-ink">{t('templates_title')}</h1>
			<button
				type="button"
				class="{buttonClasses('primary')} px-5"
				onclick={() => {
					show(NEW);
				}}
			>
				{t('templates_new')}
			</button>
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

	{#if wide || editing}
		<div class="flex min-w-0 flex-1 flex-col gap-4">
			{#if cache.snapshot !== null && editing}
				<header class="flex min-h-12 items-center gap-2">
					<button
						type="button"
						title={t('template_back')}
						aria-label={t('template_back')}
						class="grid size-11 place-items-center rounded-pill text-ink hover:bg-accent-100 lg:hidden"
						onclick={() => {
							show(null);
						}}
					>
						<Icon name="chevron-left" class="size-5.5" />
					</button>
					<h2 class="font-display text-3xl font-semibold text-ink">
						{current?.name ?? t('templates_new_title')}
					</h2>
				</header>
				{#key open}
					<TemplateForm
						data={cache.snapshot}
						{...current === undefined ? {} : { template: current }}
						onsaved={saved}
						onarchived={(isArchived) => {
							toaster.say(isArchived ? 'template_archived' : 'template_restored');
							if (isArchived) {
								show(null);
							}
						}}
						oncancel={() => {
							show(null);
						}}
					/>
				{/key}
			{:else}
				<div class="glass-card rounded-lg">
					<EmptyState title={t('templates_pick_title')} body={t('templates_pick_body')} />
				</div>
			{/if}
		</div>
	{/if}
</div>

{#if toaster.current !== null}
	{#key toaster.current.n}
		<Toast message={toaster.current.message} />
	{/key}
{/if}
