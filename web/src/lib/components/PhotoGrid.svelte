<script lang="ts">
	import { api } from '../api/app';
	import type { Media } from '../api/schemas';
	import { t } from '../i18n';
	import { layout } from '../layout';
	import { submitOnce } from '../submit';
	import BottomSheet from './BottomSheet.svelte';
	import Button from './Button.svelte';
	import Icon from './icons/Icon.svelte';
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
				class="size-full touch-pan-y object-cover select-none"
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
				<button
					type="button"
					title={t('photo_menu')}
					aria-label={t('photo_archive')}
					class="glass-card absolute top-1 right-1 grid size-11 place-items-center rounded-pill text-ink hover:bg-accent-100"
					onclick={() => {
						ask(photo);
					}}
				>
					<Icon name="archive" class="size-4.5" />
				</button>
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
		variant="secondary"
		onclick={() => {
			asking = null;
		}}
	>
		{t('cancel')}
	</Button>
</BottomSheet>
