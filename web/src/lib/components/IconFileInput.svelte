<script lang="ts">
	import { t } from '../i18n';
	import FilePicker from './FilePicker.svelte';

	// The app icon's file field (setup, Ajustes → Negocio): an "Elegir imagen" button and the name
	// of the picked file. It reports the chosen file.
	let { onpick }: { onpick: (file: File) => void } = $props();

	const labelId = $props.id();
	let picked = $state<string | null>(null);
</script>

<div role="group" aria-labelledby={labelId} class="flex flex-col gap-1.5">
	<p id={labelId} class="text-sm font-bold text-ink">{t('setup_icon_label')}</p>
	<div class="flex flex-wrap items-center gap-3">
		<div class="w-48">
			<FilePicker
				variant="ghost"
				accept="image/*"
				onpick={(file: File) => {
					picked = file.name;
					onpick(file);
				}}
			>
				{t('file_pick_image')}
			</FilePicker>
		</div>
		<p class="min-w-0 text-sm text-ink-muted">{picked ?? t('file_none')}</p>
	</div>
</div>
