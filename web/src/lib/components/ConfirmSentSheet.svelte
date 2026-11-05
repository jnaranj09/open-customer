<script lang="ts">
	import { api } from '../api/app';
	import { errorText } from '../api/errors';
	import { cache } from '../cache/store.svelte';
	import { appBadge } from '../cache/refresh';
	import { t } from '../i18n';
	import { confirmSent, notYet, pendingToAsk } from '../pending';
	import { submitOnce } from '../submit';
	import { systemNow } from '../time';
	import BottomSheet from './ui/BottomSheet.svelte';
	import Button from './ui/Button.svelte';

	// "¿Lo enviaste a …?" (design doc "Sending flow"). Mounted once in the app shell, so it opens
	// first on any app start while a confirmation is stored. It reads the stored confirmation and
	// the cached due list from the cache store and clears the confirmation only on "Sí", "Todavía no"
	// and when its item has left the due list. It cannot be dismissed (Escape, a tap outside, a
	// swipe do nothing): a dismissed question would leave the item listed with "Enviar" enabled.
	const pending = $derived(cache.meta.pending);
	// The question follows the cached server list; the badge after "Sí" counts the shown list.
	const due = $derived(cache.snapshot?.due ?? []);
	// Before the cache is read the list is empty, and an empty list would clear a real question.
	const decision = $derived(cache.snapshot === null ? 'none' : pendingToAsk(pending, due));
	const item = $derived(
		pending === null ? null : (due.find((i) => i.id === pending.item_id) ?? null)
	);

	let error = $state<string | null>(null);
	let busy = $state(false);

	$effect(() => {
		if (decision === 'clear') {
			void drop();
		}
	});

	async function drop(): Promise<void> {
		try {
			await notYet(cache);
		} catch {
			console.warn('code=cache_write_failed');
		}
	}

	const yes = submitOnce(
		async () => {
			if (pending === null || item === null) {
				return;
			}
			error = null;
			try {
				const result = await confirmSent(item, {
					api,
					store: cache,
					badge: appBadge,
					now: systemNow,
					due: cache.shownDue
				});
				if (!result.ok) {
					error = t('due_confirm_failed');
					return;
				}
			} catch {
				// The log was saved but the phone could not store it with the cleared question: the
				// item is hidden until the app closes, the question stays, and a second "Sí" is safe
				// on the server.
				console.warn('code=cache_write_failed');
				error = errorText({ kind: 'cache_write_failed' });
				return;
			}
			await cache.refresh();
		},
		(pendingNow) => {
			busy = pendingNow;
		}
	);

	const later = submitOnce(
		async () => {
			error = null;
			try {
				await notYet(cache);
			} catch {
				console.warn('code=cache_write_failed');
				error = errorText({ kind: 'cache_write_failed' });
			}
		},
		(pendingNow) => {
			busy = pendingNow;
		}
	);
</script>

<BottomSheet
	open={decision === 'ask'}
	title={t('due_confirm_sent', { name: pending?.client_name ?? '' })}
	dismissable={false}
	onclose={() => undefined}
>
	{#if item !== null}
		<p class="line-clamp-4 text-sm whitespace-pre-wrap text-ink-muted">{item.text}</p>
	{/if}
	{#if error !== null}
		<p role="alert" class="text-sm text-error">{error}</p>
	{/if}
	<Button variant="primary" {busy} onclick={() => void yes.run()}>{t('due_confirm_yes')}</Button>
	<Button variant="secondary" {busy} onclick={() => void later.run()}
		>{t('due_confirm_not_yet')}</Button
	>
</BottomSheet>
