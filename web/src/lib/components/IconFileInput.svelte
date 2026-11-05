<script lang="ts">
	import { fieldClasses } from './ui/fieldStyle';
	import { t } from '../i18n';

	// The file box of the app icon (setup, Ajustes → Negocio). It reports the chosen file, or null.
	let { onpick }: { onpick: (file: File | null) => void } = $props();
</script>

<label class="flex flex-col gap-1.5 text-sm font-bold text-ink">
	{t('setup_icon_label')}
	<input
		type="file"
		accept="image/*"
		onchange={(e) => {
			onpick(e.currentTarget.files?.[0] ?? null);
		}}
		class="{fieldClasses(false)} min-h-tap w-full p-2 font-normal"
	/>
</label>
