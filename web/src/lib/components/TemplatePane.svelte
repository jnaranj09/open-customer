<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { getContext } from 'svelte';
	import type { Template } from '../api/schemas';
	import { cache } from '../cache/store.svelte';
	import { t } from '../i18n';
	import type { createToaster } from '../toaster.svelte';
	import EmptyState from './EmptyState.svelte';
	import Icon from './icons/Icon.svelte';
	import TemplateForm from './TemplateForm.svelte';

	// The pane beside the template list: the open template's form, or the new-template form (no
	// `template`). Its routes are `/ajustes/plantillas/[id]` and `/ajustes/plantillas/nueva`, so Back
	// works; leaving with unsaved changes is asked by `TemplateForm`.
	let { template, missing = false }: { template?: Template; missing?: boolean } = $props();

	const toaster = getContext<ReturnType<typeof createToaster>>('templates-toaster');
	const list = $derived(resolve('/ajustes/plantillas'));
</script>

<div class="flex flex-col gap-4">
	<header class="flex min-h-12 items-center gap-2">
		<a
			href={list}
			title={t('template_back')}
			aria-label={t('template_back')}
			class="grid size-11 place-items-center rounded-pill text-ink hover:bg-accent-100 @3xl:hidden"
		>
			<Icon name="chevron-left" class="size-5.5" />
		</a>
		{#if template !== undefined || (!missing && cache.snapshot !== null)}
			<h2 class="font-display text-3xl font-semibold text-ink">
				{template?.name ?? t('templates_new_title')}
			</h2>
		{/if}
	</header>
	{#if missing || cache.snapshot === null}
		<div class="glass-card rounded-lg">
			<EmptyState title={t('templates_pick_title')} body={t('templates_pick_body')} />
		</div>
	{:else}
		<TemplateForm
			data={cache.snapshot}
			{...template === undefined ? {} : { template }}
			onsaved={(saved: Template) => {
				toaster.say('template_saved');
				if (template === undefined) {
					void goto(resolve('/ajustes/plantillas/[id]', { id: saved.id }));
				}
			}}
			onarchived={(isArchived: boolean) => {
				toaster.say(isArchived ? 'template_archived' : 'template_restored');
				if (isArchived) {
					void goto(list);
				}
			}}
			oncancel={() => {
				void goto(list);
			}}
		/>
	{/if}
</div>
