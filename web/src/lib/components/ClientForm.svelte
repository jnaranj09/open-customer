<script lang="ts">
	import { untrack } from 'svelte';
	import { api } from '../api/app';
	import { errorText, type ApiError } from '../api/errors';
	import type { Client } from '../api/schemas';
	import type { AttrValue } from '../booking';
	import { draftKey } from '../cache/drafts';
	import { formDraft } from '../cache/formDraft.svelte';
	import type { CacheSnapshot } from '../cache/db';
	import { cache } from '../cache/store.svelte';
	import {
		ClientFormSchema,
		clientCreateBody,
		clientFormErrors,
		clientFormFrom,
		clientPatchBody,
		restoreClientForm,
		sharingNames,
		type ClientForm
	} from '../clients';
	import { t } from '../i18n';
	import { submitOnce } from '../submit';
	import AttributeField from './AttributeField.svelte';
	import DuplicatePhone from './DuplicatePhone.svelte';
	import FormCard from './FormCard.svelte';
	import TextField from './TextField.svelte';

	// "Nueva clienta" and the detail's edit mode (design doc item 76): name and phone first, then the
	// client attributes and the notes. Creating, it keeps a draft (item 67); editing, only what
	// changed is sent. A phone another client has asks before saving ("Guardar igual").
	let {
		data,
		client,
		onsaved,
		oncancel
	}: {
		data: CacheSnapshot;
		/** The client being edited; none means a new one. */
		client?: Client;
		onsaved: (saved: Client) => void;
		oncancel?: () => void;
	} = $props();

	const creating = untrack(() => client === undefined);
	const defs = $derived(data.attributes.filter((a) => a.scope === 'client' && !a.archived));

	const initial: ClientForm = untrack(() =>
		client === undefined
			? { name: '', phone: '', notes: '', attrs: {} }
			: clientFormFrom(
					client,
					data.attributes.filter((a) => a.scope === 'client' && !a.archived)
				)
	);
	let form = $state<ClientForm>(structuredClone(initial));

	// A draft of a new client survives a reload or a closed app until it is saved.
	const draft = formDraft(
		creating ? draftKey('clienta', 'new') : null,
		ClientFormSchema,
		initial,
		() => $state.snapshot(form),
		(saved) => {
			form = restoreClientForm(saved, new Set(defs.map((a) => a.key)));
		}
	);

	let attempted = $state(false);
	const errors = $derived(attempted ? clientFormErrors(form) : {});
	let failure = $state<ApiError | null>(null);
	let busy = $state(false);
	const sharing = $derived(sharingNames(failure));

	const fieldError = (name: string): string | undefined =>
		failure?.kind === 'problem' ? failure.fields[name] : undefined;

	const attrValue = (key: string, multi: boolean): AttrValue =>
		form.attrs[key] ?? (multi ? [] : '');
	const setterFor =
		(key: string) =>
		(v: AttrValue): void => {
			form.attrs[key] = v;
		};

	const submit = submitOnce(
		async (allowDuplicate: boolean) => {
			failure = null;
			attempted = true;
			if (Object.keys(clientFormErrors(form)).length > 0) {
				return;
			}
			const extra = allowDuplicate ? { allow_duplicate_phone: true } : {};
			let result;
			if (client === undefined) {
				result = await api.createClient({ ...clientCreateBody(form, defs), ...extra });
			} else {
				const patch = clientPatchBody(client, form, defs);
				if (Object.keys(patch).length === 0) {
					onsaved(client);
					return;
				}
				result = await api.updateClient(client.id, { ...patch, ...extra });
			}
			if (!result.ok) {
				failure = result.error;
				return;
			}
			await draft.clear();
			await cache.refresh();
			onsaved(result.data);
		},
		(pending) => {
			busy = pending;
		}
	);
</script>

<FormCard
	restored={draft.restored}
	label={creating ? t('client_save') : t('client_save_changes')}
	{busy}
	onsubmit={() => {
		void submit.run(false);
	}}
	onstartover={() => {
		void draft.startOver();
	}}
	{oncancel}
>
	<div class="grid gap-4 lg:grid-cols-2 lg:gap-8">
		<TextField
			label={t('client_name')}
			autocomplete="off"
			bind:value={form.name}
			error={errors.name === undefined ? fieldError('name') : t(errors.name)}
		/>
		<TextField
			label={t('client_phone')}
			type="tel"
			autocomplete="off"
			bind:value={form.phone}
			error={fieldError('phone')}
		/>
	</div>

	<div class="grid gap-4 lg:grid-cols-2 lg:gap-8">
		{#each defs as attr (attr.id)}
			<AttributeField
				{attr}
				bind:value={() => attrValue(attr.key, attr.type === 'multiselect'), setterFor(attr.key)}
				error={fieldError(`attrs.${attr.key}`)}
			/>
		{/each}
		<TextField label={t('client_notes')} bind:value={form.notes} error={fieldError('notes')} />
	</div>

	{#if sharing !== null}
		<DuplicatePhone
			names={sharing}
			{busy}
			label={t('client_save_anyway')}
			onconfirm={() => {
				void submit.run(true);
			}}
		/>
	{:else if failure !== null}
		<p role="alert" class="text-sm text-error">{errorText(failure)}</p>
	{/if}
</FormCard>
