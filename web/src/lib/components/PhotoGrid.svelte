<script lang="ts">
	import { api } from '../api/app';
	import type { Media } from '../api/schemas';
	import { t } from '../i18n';
	import { layout } from '../layout';
	import { submitOnce } from '../submit';
	import BottomSheet from './ui/BottomSheet.svelte';
	import Button from './ui/Button.svelte';
	import IconButton from './ui/IconButton.svelte';
	import WriteButton from './WriteButton.svelte';

	// A grid of photo thumbnails: 3 columns on compact, 5 on wide. "Archivar foto" comes from a
	// long-press on compact and from the tile's menu button on wide, and always asks first. The
	// caller does the archiving and answers with the error text, or null when it worked.
	let {
		photos,
		onarchive
	}: { photos: Media[]; onarchive: (photo: Media) => Promise<string | null> } = $props();

	const LONG_PRESS_MS = 500;

	let asking = $state<Media | null>(null);
	let failure = $state<string | null>(null);
	let busy = $state(false);
	let timer: ReturnType<typeof setTimeout> | undefined;

	function ask(photo: Media): void {
		failure = null;
		asking = photo;
	}

	function press(photo: Media): void {
		clearTimeout(timer);
		timer = setTimeout(() => {
			ask(photo);
		}, LONG_PRESS_MS);
	}

	function release(): void {
		clearTimeout(timer);
	}

	const archive = submitOnce(
		async (photo: Media) => {
			failure = await onarchive(photo);
			if (failure === null) {
				asking = null;
			}
		},
		(pending) => {
			busy = pending;
		}
	);
</script>

<ul class="grid grid-cols-3 gap-2 lg:grid-cols-5">
	{#each photos as photo (photo.id)}
		<li class="relative aspect-square overflow-hidden rounded-md">
			<img
				src={api.mediaThumbUrl(photo.id)}
				alt={t('photo_alt')}
				loading="lazy"
				draggable="false"
				class="size-full touch-pan-y object-cover select-none touch-callout-none"
				onpointerdown={() => {
					press(photo);
				}}
				onpointerup={release}
				onpointerleave={release}
				onpointercancel={release}
				oncontextmenu={(e) => {
					e.preventDefault();
				}}
			/>
			{#if layout.current === 'wide'}
				<IconButton
					variant="glass"
					icon="archive"
					label={t('photo_archive')}
					class="absolute top-1 right-1"
					onclick={() => {
						ask(photo);
					}}
				/>
			{/if}
		</li>
	{/each}
</ul>

<BottomSheet
	open={asking !== null}
	title={t('photo_archive')}
	onclose={() => {
		asking = null;
	}}
>
	<p class="text-base text-ink">{t('photo_archive_body')}</p>
	{#if failure !== null}
		<p role="alert" class="text-sm text-error">{failure}</p>
	{/if}
	<WriteButton
		variant="danger"
		{busy}
		onclick={() => {
			if (asking !== null) {
				void archive.run(asking);
			}
		}}
	>
		{t('photo_archive')}
	</WriteButton>
	<Button
		variant="glass"
		onclick={() => {
			asking = null;
		}}
	>
		{t('cancel')}
	</Button>
</BottomSheet>
