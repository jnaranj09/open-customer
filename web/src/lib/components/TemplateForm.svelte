<script lang="ts">
	import { untrack } from 'svelte';
	import { beforeNavigate, goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import type { Path } from '$app/types';
	import { api } from '../api/app';
	import { errorText, type ApiError } from '../api/errors';
	import type { Template } from '../api/schemas';
	import type { CacheSnapshot } from '../cache/db';
	import { cache } from '../cache/store.svelte';
	import type { BodyProblem } from '../chips';
	import { t } from '../i18n';
	import { navigationAllowed, templateDirty, type Exemption } from '../templateEdit';
	import { submitOnce } from '../submit';
	import ActionButton from './ActionButton.svelte';
	import BottomSheet from './BottomSheet.svelte';
	import Button from './Button.svelte';
	import ChipEditor from './ChipEditor.svelte';
	import FormCard from './FormCard.svelte';
	import TextField from './TextField.svelte';

	// One message template: its name and its text in the ChipEditor (design doc item 77). The
	// server checks the body on save, and its answer is shown by field. The caller keys the form on
	// the template, so another one starts with its own name and text.
	let {
		data,
		template,
		onsaved,
		onarchived,
		oncancel
	}: {
		data: CacheSnapshot;
		/** The template being edited; none means a new one. */
		template?: Template;
		onsaved: (saved: Template) => void;
		/** Told after the template was archived or restored. */
		onarchived: (archived: boolean) => void;
		oncancel?: () => void;
	} = $props();

	let name = $state(untrack(() => template?.name ?? ''));
	let body = $state(untrack(() => template?.body ?? ''));
	let problems = $state<BodyProblem[]>([]);
	let attempted = $state(false);
	// Leaving with unsaved changes asks first, unless `navigationAllowed` says the form itself is
	// about to navigate (a create, an archive) or she chose to discard. An update or a restore stays
	// on the page and sets no exemption.
	let exemption: Exemption | null = null;
	let discardTo = $state<URL | null>(null);
	const dirty = $derived(templateDirty(template, name, body));

	beforeNavigate((nav) => {
		if (navigationAllowed(exemption, dirty, name, body)) {
			return;
		}
		nav.cancel();
		// A reload or a closed tab gets the browser's own question; a move inside the app gets ours.
		if (!nav.willUnload && nav.to !== null) {
			discardTo = nav.to.url;
		}
	});
	let failure = $state<ApiError | null>(null);
	let busy = $state(false);
	let archiving = $state(false);
	let toggling = $state(false);

	const locked = $derived(template?.archived === true);
	const fieldError = (field: string): string | undefined =>
		failure?.kind === 'problem' ? failure.fields[field] : undefined;
	const nameError = $derived(
		attempted && name.trim() === '' ? t('template_name_required') : fieldError('name')
	);
	const bodyError = $derived(
		attempted && body === '' ? t('template_message_required') : fieldError('body')
	);

	const submit = submitOnce(
		async () => {
			failure = null;
			attempted = true;
			if (name.trim() === '' || body === '' || problems.length > 0) {
				return;
			}
			const trimmed = name.trim();
			const sent = body;
			if (template !== undefined && trimmed === template.name && body === template.body) {
				onsaved(template);
				return;
			}
			const result =
				template === undefined
					? await api.createTemplate({ name: trimmed, body })
					: await api.updateTemplate(template.id, {
							...(trimmed === template.name ? {} : { name: trimmed }),
							...(body === template.body ? {} : { body })
						});
			if (!result.ok) {
				failure = result.error;
				return;
			}
			await cache.refresh();
			if (template === undefined) {
				exemption = { kind: 'saved', name: trimmed, body: sent };
			}
			onsaved(result.data);
		},
		(pending) => {
			busy = pending;
		}
	);

	const setArchived = submitOnce(
		async (archived: boolean) => {
			if (template === undefined) {
				return;
			}
			failure = null;
			const sent = { name: name.trim(), body };
			const result = await api.updateTemplate(template.id, { archived });
			if (!result.ok) {
				failure = result.error;
				archiving = false;
				return;
			}
			await cache.refresh();
			archiving = false;
			if (archived) {
				exemption = { kind: 'saved', ...sent };
			}
			onarchived(archived);
		},
		(pending) => {
			toggling = pending;
		}
	);

	function discard(): void {
		const to = discardTo;
		discardTo = null;
		exemption = { kind: 'discard' };
		if (to !== null) {
			// The app's own address, as a typed path: `resolve` adds the base path to it.
			void goto(resolve((to.pathname + to.search) as Path));
		}
	}

	const otherError = $derived(
		failure === null || fieldError('name') !== undefined || fieldError('body') !== undefined
			? null
			: errorText(failure)
	);
</script>

<FormCard
	label={t('template_save')}
	{busy}
	onsubmit={() => {
		void submit.run();
	}}
	{oncancel}
>
	{#if locked}
		<p class="rounded-md bg-glass-tint px-4 py-3 text-sm text-ink">{t('template_archived_note')}</p>
	{/if}
	<TextField label={t('template_name')} autocomplete="off" bind:value={name} error={nameError} />
	<ChipEditor
		value={body}
		vars={cache.meta.templateVars}
		attributes={data.attributes}
		label={t('template_message')}
		error={bodyError}
		onchange={(next: string, found: BodyProblem[]) => {
			body = next;
			problems = found;
		}}
	/>
	{#if otherError !== null}
		<p role="alert" class="text-sm text-error">{otherError}</p>
	{/if}
	{#if template !== undefined}
		<div class="flex lg:max-w-64">
			{#if locked}
				<ActionButton
					variant="secondary"
					busy={toggling}
					onclick={() => {
						void setArchived.run(false);
					}}
				>
					{t('template_restore')}
				</ActionButton>
			{:else}
				<ActionButton
					variant="secondary"
					onclick={() => {
						archiving = true;
					}}
				>
					{t('template_archive')}
				</ActionButton>
			{/if}
		</div>
	{/if}
</FormCard>

<BottomSheet
	open={archiving}
	title={t('template_archive')}
	onclose={() => {
		archiving = false;
	}}
>
	<p class="text-base text-ink">{t('template_archive_body')}</p>
	<Button
		variant="danger"
		busy={toggling}
		onclick={() => {
			void setArchived.run(true);
		}}
	>
		{t('template_archive')}
	</Button>
	<Button
		variant="secondary"
		onclick={() => {
			archiving = false;
		}}
	>
		{t('cancel')}
	</Button>
</BottomSheet>

<BottomSheet
	open={discardTo !== null}
	title={t('template_discard_title')}
	onclose={() => {
		discardTo = null;
	}}
>
	<p class="text-base text-ink">{t('template_discard_body')}</p>
	<Button variant="danger" onclick={discard}>{t('template_discard')}</Button>
	<Button
		variant="secondary"
		onclick={() => {
			discardTo = null;
		}}
	>
		{t('template_keep_editing')}
	</Button>
</BottomSheet>
