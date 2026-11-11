<script lang="ts">
	import { untrack } from 'svelte';
	import { api } from '../api/app';
	import { errorText, type ApiError } from '../api/errors';
	import type { QuickReply } from '../api/schemas';
	import { reorder } from '../attributes';
	import { cache } from '../cache/store.svelte';
	import { t, type MessageKey } from '../i18n';
	import { submitOnce } from '../submit';
	import ActionButton from './ActionButton.svelte';
	import MoveButtons from './MoveButtons.svelte';
	import SaveButton from './SaveButton.svelte';
	import BottomSheet from './ui/BottomSheet.svelte';
	import Button from './ui/Button.svelte';
	import TextField from './ui/TextField.svelte';

	// Add or edit one quick reply in a sheet (reminders-and-replies design, "Quick replies"). The
	// page mounts it when she opens a reply, so every opening starts from the saved values. An
	// active reply moves up or down among the active ones: one PATCH per moved row, as the
	// attribute options do. The server checks the title and the body; its answer shows by field.
	let {
		reply,
		active,
		onclose,
		ondone
	}: {
		/** The reply being edited; none means a new one. */
		reply?: QuickReply;
		/** The active replies in her order, for the moves. */
		active: readonly QuickReply[];
		onclose: () => void;
		/** Told after a save, an archive or a restore, with the message to show. */
		ondone: (message: MessageKey) => void;
	} = $props();

	let title = $state(untrack(() => reply?.title ?? ''));
	let body = $state(untrack(() => reply?.body ?? ''));
	let failure = $state<ApiError | null>(null);
	let busy = $state(false);
	let working = $state(false);

	const fieldError = (key: string): string | undefined =>
		failure?.kind === 'problem' ? failure.fields[key] : undefined;
	const otherError = $derived(
		failure === null || fieldError('title') !== undefined || fieldError('body') !== undefined
			? null
			: errorText(failure)
	);
	const index = $derived(reply === undefined ? -1 : active.findIndex((r) => r.id === reply.id));

	const save = submitOnce(
		async () => {
			failure = null;
			if (reply === undefined) {
				const result = await api.createQuickReply({ title, body });
				if (!result.ok) {
					failure = result.error;
					return;
				}
				await cache.refresh();
				ondone('reply_created');
				return;
			}
			const patch = {
				...(title === reply.title ? {} : { title }),
				...(body === reply.body ? {} : { body })
			};
			if (Object.keys(patch).length > 0) {
				const result = await api.updateQuickReply(reply.id, patch);
				if (!result.ok) {
					failure = result.error;
					return;
				}
				await cache.refresh();
			}
			ondone('reply_saved');
		},
		(pending) => {
			busy = pending;
		}
	);

	// Rows are sent one by one, so a failure leaves the earlier ones saved: refresh either way.
	const move = submitOnce(
		async (to: number) => {
			failure = null;
			for (const step of reorder(active, index, to)) {
				const result = await api.updateQuickReply(step.id, { position: step.position });
				if (!result.ok) {
					failure = result.error;
					break;
				}
			}
			await cache.refresh();
		},
		(pending) => {
			working = pending;
		}
	);

	const setArchived = submitOnce(
		async (archived: boolean) => {
			if (reply === undefined) {
				return;
			}
			failure = null;
			const result = await api.updateQuickReply(reply.id, { archived });
			if (!result.ok) {
				failure = result.error;
				return;
			}
			await cache.refresh();
			ondone(archived ? 'reply_archived' : 'reply_restored');
		},
		(pending) => {
			working = pending;
		}
	);
</script>

<BottomSheet open title={reply === undefined ? t('replies_new') : reply.title} {onclose}>
	<form
		class="flex flex-col gap-4"
		onsubmit={(e) => {
			e.preventDefault();
			void save.run();
		}}
	>
		<TextField
			label={t('reply_title')}
			bind:value={title}
			autocomplete="off"
			error={fieldError('title')}
		/>
		<TextField label={t('reply_body')} bind:value={body} lines={6} error={fieldError('body')} />
		{#if otherError !== null}
			<p role="alert" class="text-sm text-error">{otherError}</p>
		{/if}
		<SaveButton {busy}>{t('reply_save')}</SaveButton>
	</form>
	{#if reply !== undefined}
		{#if reply.archived}
			<div class="flex">
				<ActionButton
					variant="glass"
					busy={working}
					onclick={() => {
						void setArchived.run(false);
					}}>{t('reply_restore')}</ActionButton
				>
			</div>
		{:else}
			<div role="group" aria-label={reply.title} class="flex flex-wrap gap-1">
				<MoveButtons
					{index}
					count={active.length}
					disabled={working}
					onmove={(to: number) => {
						void move.run(to);
					}}
				/>
			</div>
			<div class="flex">
				<ActionButton
					variant="glass"
					busy={working}
					onclick={() => {
						void setArchived.run(true);
					}}>{t('reply_archive')}</ActionButton
				>
			</div>
		{/if}
	{/if}
	<Button variant="ghost" onclick={onclose}>{t('cancel')}</Button>
</BottomSheet>
