<script lang="ts">
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
		class="min-h-tap w-full rounded-sm border border-glass-edge bg-glass-strong p-2 text-base font-normal"
	/>
</label>
