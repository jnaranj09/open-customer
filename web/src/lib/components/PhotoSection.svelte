<script lang="ts">
	import Card from './ui/Card.svelte';
	import { buttonClasses } from './ui/buttonStyle';
	import { untrack } from 'svelte';
	import { api } from '../api/app';
	import { errorText } from '../api/errors';
	import type { Media } from '../api/schemas';
	import { cache } from '../cache/store.svelte';
	import { t, type MessageKey } from '../i18n';
	import { toJpegs } from '../photos';
	import { submitOnce } from '../submit';
	import { createToaster } from '../toaster.svelte';
	import Icon from './icons/Icon.svelte';
	import PhotoGrid from './PhotoGrid.svelte';
	import Toast from './Toast.svelte';

	// The photos of one client (the gallery) or of one of her appointments (design doc "Photos"): the
	// list from the server, "Agregar foto" and "Archivar foto". Give it a new key when the owner
	// changes: it reads its list once, on open.
	let {
		clientId,
		appointmentId,
		empty
	}: {
		clientId: string;
		/** Only the photos taken at this appointment; none means the client's whole gallery. */
		appointmentId?: string;
		/** The text under the title while there is no photo. */
		empty: MessageKey;
	} = $props();

	let photos = $state<Media[] | null>(null);
	// The list's own failure and the last upload's, apart: reloading the list must not wipe an
	// upload error.
	let listError = $state<string | null>(null);
	let photosError = $state<string | null>(null);
	let uploading = $state(false);
	const toaster = createToaster();

	async function load(): Promise<void> {
		const result = await api.listClientMedia(clientId);
		if (result.ok) {
			photos = result.data.filter(
				(m) => appointmentId === undefined || m.appointment_id === appointmentId
			);
			listError = null;
		} else {
			listError = result.error.kind === 'offline' ? t('photos_offline') : errorText(result.error);
		}
	}

	// Syncs with the server: reads the list once when the section opens.
	$effect(() => {
		untrack(() => {
			void load();
		});
	});

	const upload = submitOnce(
		async (file: File) => {
			photosError = null;
			let jpegs;
			try {
				jpegs = await toJpegs(file);
			} catch {
				console.warn('code=photo_decode_failed');
				photosError = t('photo_unreadable');
				return;
			}
			const result = await api.uploadMedia({
				...jpegs,
				...(appointmentId === undefined
					? { client_id: clientId }
					: { appointment_id: appointmentId })
			});
			if (!result.ok) {
				photosError = errorText(result.error);
				return;
			}
			await load();
			toaster.say('photo_added');
		},
		(pending) => {
			uploading = pending;
		}
	);

	function picked(e: Event & { currentTarget: HTMLInputElement }): void {
		const file = e.currentTarget.files?.[0];
		e.currentTarget.value = '';
		if (file !== undefined) {
			void upload.run(file);
		}
	}

	async function archive(photo: Media): Promise<string | null> {
		const result = await api.archiveMedia(photo.id);
		if (!result.ok) {
			return errorText(result.error);
		}
		photos = (photos ?? []).filter((m) => m.id !== photo.id);
		toaster.say('photo_archived');
		return null;
	}
</script>

<Card as="section" radius="xl" pad="section" class="flex flex-col gap-3">
	<div class="flex items-center gap-3">
		<h2 class="flex-1 text-xl font-bold text-ink">{t('photos_title')}</h2>
		<label
			class="{buttonClasses('ghost')} relative focus-within:outline-2 {cache.offline || uploading
				? 'pointer-events-none opacity-60'
				: ''}"
		>
			<Icon name="plus" />
			{uploading ? t('photo_adding') : t('photo_add')}
			<input
				type="file"
				accept="image/*"
				disabled={cache.offline || uploading}
				onchange={picked}
				class="absolute inset-0 size-full cursor-pointer opacity-0"
			/>
		</label>
	</div>
	{#if photosError !== null || listError !== null}
		<p role="alert" class="text-sm text-error">{photosError ?? listError}</p>
	{:else if photos !== null && photos.length === 0}
		<p class="text-sm text-ink-muted">{t(empty)}</p>
	{/if}
	{#if photos !== null && photos.length > 0}
		<PhotoGrid {photos} onarchive={archive} />
	{/if}
</Card>

{#if toaster.current !== null}
	{#key toaster.current.n}
		<Toast message={toaster.current.message} />
	{/key}
{/if}
