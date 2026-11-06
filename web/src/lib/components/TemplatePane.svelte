<script lang="ts">
	import Card from './ui/Card.svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { getContext } from 'svelte';
	import type { Template } from '../api/schemas';
	import { cache } from '../cache/store.svelte';
	import { t } from '../i18n';
	import type { createToaster } from '../toaster.svelte';
	import EmptyState from './ui/EmptyState.svelte';
	import IconButton from './ui/IconButton.svelte';
	import TemplateForm from './TemplateForm.svelte';

	// The pane beside the template list: the open template's form, or the new-template form (no
	// `template`). Its routes are `/ajustes/plantillas/[id]` and `/ajustes/plantillas/nueva`, so Back
	// works; leaving with unsaved changes is asked by `TemplateForm`. Leaving the new-template form
	// by Save or Cancel replaces its history entry, so Back does not open an empty form again.
	let { template, missing = false }: { template?: Template; missing?: boolean } = $props();

	const toaster = getContext<ReturnType<typeof createToaster>>('templates-toaster');
	const list = $derived(resolve('/ajustes/plantillas'));
</script>

<div class="flex flex-col gap-4">
	<header class="flex min-h-12 items-center gap-2">
		<IconButton
			variant="plain"
			icon="chevron-left"
			label={t('template_back')}
			href={list}
			class="@3xl:hidden"
		/>
		{#if template !== undefined || (!missing && cache.snapshot !== null)}
			<h2 class="text-3xl font-extrabold text-ink">
				{template?.name ?? t('templates_new_title')}
			</h2>
		{/if}
	</header>
	{#if missing || cache.snapshot === null}
		<Card>
			<EmptyState title={t('templates_pick_title')} body={t('templates_pick_body')} />
		</Card>
	{:else}
		<TemplateForm
			data={cache.snapshot}
			{...template === undefined ? {} : { template }}
			onsaved={(saved: Template) => {
				toaster.say('template_saved');
				if (template === undefined) {
					void goto(resolve('/ajustes/plantillas/[id]', { id: saved.id }), { replaceState: true });
				}
			}}
			onarchived={(isArchived: boolean) => {
				toaster.say(isArchived ? 'template_archived' : 'template_restored');
				if (isArchived) {
					void goto(list);
				}
			}}
			oncancel={() => {
				void goto(list, { replaceState: template === undefined });
			}}
		/>
	{/if}
</div>
