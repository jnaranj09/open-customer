<script lang="ts">
	import Chip from './ui/Chip.svelte';
	import { PREVIEW_DEBOUNCE_MS } from '../preview';
	import { untrack } from 'svelte';
	import { api } from '../api/app';
	import { errorText, type ApiError } from '../api/errors';
	import { cache } from '../cache/store.svelte';
	import { t } from '../i18n';
	import ClientPicker from './ClientPicker.svelte';

	// What the message looks like, rendered by the server (one renderer), 300 ms after the last
	// change. "Vista previa con:" picks the sample client or one of hers (design doc item 70).
	let { body }: { body: string } = $props();

	let mode = $state<'sample' | 'client'>('sample');
	let clientId = $state<string | null>(null);
	let text = $state<string | null>(null);
	let failure = $state<ApiError | null>(null);
	let updating = $state(false);

	const clients = $derived(cache.snapshot?.clients ?? []);

	// Timers and the server are outside systems, so this is an effect. A change before the answer
	// comes marks that answer stale: it is dropped, never shown after a newer one.
	$effect(() => {
		const current = body;
		const id = mode === 'client' ? clientId : null;
		if (current === '') {
			text = null;
			failure = null;
			updating = false;
			return;
		}
		// The first answer does not wait; later ones wait for her to stop typing.
		const wait = untrack(() => (text === null && failure === null ? 0 : PREVIEW_DEBOUNCE_MS));
		updating = true;
		let stale = false;
		const timer = setTimeout(() => {
			void api
				.previewTemplate({ body: current, ...(id === null ? {} : { client_id: id }) })
				.then((result) => {
					if (stale) {
						return;
					}
					updating = false;
					if (result.ok) {
						text = result.data.text;
						failure = null;
					} else {
						failure = result.error;
					}
				});
		}, wait);
		return () => {
			stale = true;
			clearTimeout(timer);
		};
	});
</script>

<section class="flex flex-col gap-3" aria-labelledby="preview-with">
	<div class="flex flex-wrap items-center gap-2">
		<h3 id="preview-with" class="text-sm font-bold text-ink">{t('preview_with')}</h3>
		<div class="flex gap-2">
			{#each [['sample', 'preview_sample'], ['client', 'preview_client']] as const as [value, label] (value)}
				<Chip
					variant={mode === value ? 'selected' : 'default'}
					onclick={() => {
						mode = value;
					}}
				>
					{t(label)}
				</Chip>
			{/each}
		</div>
	</div>

	{#if mode === 'client'}
		<ClientPicker {clients} bind:value={clientId} allowAdd={false} />
	{/if}

	<div
		role="status"
		aria-live="polite"
		class="min-h-24 rounded-lg bg-glass-tint px-5 py-4 text-base leading-relaxed wrap-break-word whitespace-pre-wrap text-ink"
	>
		{#if text === null}
			<span class="text-ink-muted">{body === '' ? t('preview_empty') : t('preview_updating')}</span>
		{:else}
			{text}
		{/if}
	</div>
	{#if failure !== null}
		<p role="alert" class="text-sm text-error">{errorText(failure)}</p>
	{:else if updating && text !== null}
		<p class="text-sm text-ink-muted">{t('preview_updating')}</p>
	{/if}
</section>
