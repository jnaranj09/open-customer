<script lang="ts">
	import Callout from './ui/Callout.svelte';
	import { getContext, untrack } from 'svelte';
	import { api } from '../api/app';
	import { errorText, type ApiError, type Result } from '../api/errors';
	import type { Attribute } from '../api/schemas';
	import {
		archiveWarning,
		hasOptions,
		SCOPE_LABELS,
		TYPE_LABELS,
		type Positioned
	} from '../attributes';
	import { cache } from '../cache/store.svelte';
	import { t, type MessageKey } from '../i18n';
	import { submitOnce } from '../submit';
	import type { createToaster } from '../toaster.svelte';
	import ActionButton from './ActionButton.svelte';
	import BottomSheet from './ui/BottomSheet.svelte';
	import Button from './ui/Button.svelte';
	import OptionTree from './OptionTree.svelte';
	import SaveButton from './SaveButton.svelte';
	import SelectField from './ui/SelectField.svelte';
	import TextField from './ui/TextField.svelte';
	import WriteButton from './WriteButton.svelte';

	// Add or edit one attribute in a sheet (design doc item 77, CUJ 2 step 2). A new one asks for
	// its name, the plain scope question and its type; once it exists the sheet shows the same
	// attribute for editing, with its options when it is a select. Type and scope are fixed after
	// creation (the API refuses them). Archiving asks first and lists the templates that use it.
	let {
		attribute,
		onclose,
		oncreated,
		ondone
	}: {
		/** The attribute being edited; none means a new one. */
		attribute?: Attribute;
		onclose: () => void;
		/** Told with the id of a new select attribute, so the page can reopen the sheet on it to add options. */
		oncreated: (id: string) => void;
		/** Told after a save, an archive or a restore, with the message to show. */
		ondone: (message: MessageKey) => void;
	} = $props();

	const toaster = getContext<ReturnType<typeof createToaster>>('settings-toaster');

	let label = $state(untrack(() => attribute?.label ?? ''));
	let required = $state(untrack(() => attribute?.required ?? false));
	let scope = $state<Attribute['scope']>('client');
	let type = $state<Attribute['type']>('text');
	let attempted = $state(false);
	let failure = $state<ApiError | null>(null);
	let optionError = $state<string | null>(null);
	let busy = $state(false);
	let toggling = $state(false);
	let confirming = $state(false);

	const templates = $derived(
		attribute === undefined ? [] : archiveWarning(attribute, cache.snapshot?.templates ?? [])
	);
	const fieldError = (field: string): string | undefined =>
		failure?.kind === 'problem' ? failure.fields[field] : undefined;
	const labelError = $derived(
		attempted && label.trim() === '' ? t('attribute_label_required') : fieldError('label')
	);
	const otherError = $derived(
		failure === null || (failure.kind === 'problem' && Object.keys(failure.fields).length > 0)
			? null
			: errorText(failure)
	);

	const typeOptions = (Object.keys(TYPE_LABELS) as Attribute['type'][]).map((id) => ({
		id,
		label: t(TYPE_LABELS[id])
	}));

	const save = submitOnce(
		async () => {
			failure = null;
			attempted = true;
			if (label.trim() === '') {
				return;
			}
			if (attribute === undefined) {
				const result = await api.createAttribute({ label: label.trim(), scope, type, required });
				if (!result.ok) {
					failure = result.error;
					return;
				}
				await cache.refresh();
				// A select goes on to its options; the other types are done.
				if (hasOptions(result.data.type)) {
					oncreated(result.data.id);
				} else {
					ondone('attribute_created');
				}
				return;
			}
			const result = await api.updateAttribute(attribute.id, {
				...(label.trim() === attribute.label ? {} : { label: label.trim() }),
				...(required === attribute.required ? {} : { required })
			});
			if (!result.ok) {
				failure = result.error;
				return;
			}
			await cache.refresh();
			ondone('attribute_saved');
		},
		(pending) => {
			busy = pending;
		}
	);

	const setArchived = submitOnce(
		async (archived: boolean) => {
			if (attribute === undefined) {
				return;
			}
			failure = null;
			const result = await api.updateAttribute(attribute.id, { archived });
			if (!result.ok) {
				failure = result.error;
				confirming = false;
				return;
			}
			await cache.refresh();
			ondone(archived ? 'attribute_archived' : 'attribute_restored');
		},
		(pending) => {
			toggling = pending;
		}
	);

	/** Refreshes after an option call and tells whether it worked; a failure shows under the tree. */
	async function settled(result: Result<unknown>, message?: MessageKey): Promise<boolean> {
		optionError = result.ok ? null : errorText(result.error);
		if (result.ok) {
			await cache.refresh();
			if (message !== undefined) {
				toaster.say(message);
			}
		}
		return result.ok;
	}

	const addOption = async (parent: string | null, text: string): Promise<boolean> =>
		attribute === undefined
			? false
			: settled(
					await api.createOption(attribute.id, {
						label: text,
						...(parent === null ? {} : { parent_id: parent })
					}),
					'option_added'
				);

	const renameOption = async (id: string, text: string): Promise<boolean> =>
		attribute === undefined
			? false
			: settled(await api.updateOption(attribute.id, id, { label: text }), 'option_saved');

	/** Archives at once; the toast's "Deshacer" sends the option back (`archived: false`). */
	async function archiveOption(id: string): Promise<boolean> {
		if (attribute === undefined) {
			return false;
		}
		const attributeId = attribute.id;
		const ok = await settled(await api.updateOption(attributeId, id, { archived: true }));
		if (ok) {
			toaster.say('option_archived', {
				label: 'due_undo',
				run: () => {
					// The toast can outlive the connection: undoing writes, so offline it says why.
					if (cache.offline) {
						optionError = t('offline_cannot_save');
						return;
					}
					void api.updateOption(attributeId, id, { archived: false }).then(async (back) => {
						optionError = back.ok ? null : errorText(back.error);
						if (back.ok) {
							await cache.refresh();
						}
					});
				}
			});
		}
		return ok;
	}

	/** One `PATCH` per row that moved; the first failure stops the rest. The cache is refreshed
	 *  either way, so the tree shows the order the server has. */
	async function moveOptions(moves: Positioned[]): Promise<boolean> {
		if (attribute === undefined) {
			return false;
		}
		let failed: ApiError | null = null;
		for (const move of moves) {
			const result = await api.updateOption(attribute.id, move.id, { position: move.position });
			if (!result.ok) {
				failed = result.error;
				break;
			}
		}
		// Rows are sent one by one, so a failure leaves the earlier ones saved: refresh either way.
		await cache.refresh();
		if (failed !== null) {
			optionError = errorText(failed);
			return false;
		}
		optionError = null;
		toaster.say('option_moved');
		return true;
	}

	const title = $derived(
		attribute === undefined
			? t('attribute_new_title')
			: confirming
				? t('attribute_archive_title', { label: attribute.label })
				: attribute.label
	);
</script>

<BottomSheet open {title} {onclose}>
	{#if confirming && attribute !== undefined}
		<p class="text-base text-ink">{t('attribute_archive_body')}</p>
		{#if templates.length > 0}
			<Callout class="text-sm">
				<p class="font-bold">{t('attribute_archive_templates')}</p>
				<ul class="mt-1 list-disc pl-5">
					{#each templates as name (name)}
						<li>{name}</li>
					{/each}
				</ul>
			</Callout>
		{/if}
		{#if failure !== null}
			<p role="alert" class="text-sm text-error">{errorText(failure)}</p>
		{/if}
		<WriteButton
			variant="danger"
			busy={toggling}
			onclick={() => {
				void setArchived.run(true);
			}}
		>
			{t('attribute_archive')}
		</WriteButton>
		<Button
			variant="glass"
			onclick={() => {
				confirming = false;
			}}
		>
			{t('cancel')}
		</Button>
	{:else}
		<form
			class="flex flex-col gap-4"
			onsubmit={(e) => {
				e.preventDefault();
				void save.run();
			}}
		>
			<TextField
				label={t('attribute_label')}
				bind:value={label}
				autocomplete="off"
				error={labelError}
			/>
			{#if attribute === undefined}
				<fieldset class="flex flex-col gap-1">
					<legend class="mb-1 text-sm font-bold text-ink">{t('attribute_scope_question')}</legend>
					{#each ['client', 'appointment'] as const as choice (choice)}
						<label class="flex min-h-tap items-center gap-3 text-base text-ink">
							<input
								type="radio"
								name="scope"
								value={choice}
								bind:group={scope}
								class="size-5 accent-accent"
							/>
							{t(choice === 'client' ? 'attribute_scope_client' : 'attribute_scope_appointment')}
						</label>
					{/each}
				</fieldset>
				<SelectField
					label={t('attribute_type')}
					bind:value={type}
					options={typeOptions}
					error={fieldError('type')}
				/>
			{:else}
				<p class="text-sm text-ink-muted">
					{t(TYPE_LABELS[attribute.type])} · {t(SCOPE_LABELS[attribute.scope])}. {t(
						'attribute_fixed_note'
					)}
				</p>
			{/if}
			<label class="flex min-h-tap items-center gap-3 text-base text-ink">
				<input type="checkbox" bind:checked={required} class="size-5 accent-accent" />
				{t('attribute_required')}
			</label>
			{#if otherError !== null}
				<p role="alert" class="text-sm text-error">{otherError}</p>
			{/if}
			<SaveButton {busy}>
				{attribute === undefined ? t('attribute_create') : t('attribute_save')}
			</SaveButton>
		</form>
		{#if attribute !== undefined}
			{#if hasOptions(attribute.type) && !attribute.archived}
				<section class="flex flex-col gap-2">
					<h3 class="text-sm font-bold text-ink">{t('options_title')}</h3>
					<OptionTree
						options={attribute.options}
						onadd={addOption}
						onrename={renameOption}
						onmove={moveOptions}
						onarchive={archiveOption}
					/>
					{#if optionError !== null}
						<p role="alert" class="text-sm text-error">{optionError}</p>
					{/if}
				</section>
			{/if}
			<div class="flex">
				{#if attribute.archived}
					<ActionButton
						variant="glass"
						busy={toggling}
						onclick={() => {
							void setArchived.run(false);
						}}
					>
						{t('attribute_restore')}
					</ActionButton>
				{:else}
					<ActionButton
						variant="glass"
						onclick={() => {
							confirming = true;
						}}
					>
						{t('attribute_archive')}
					</ActionButton>
				{/if}
			</div>
		{/if}
		<Button variant="ghost" onclick={onclose}>{t('cancel')}</Button>
	{/if}
</BottomSheet>
