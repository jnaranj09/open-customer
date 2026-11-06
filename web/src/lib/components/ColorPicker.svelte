<script lang="ts">
	import { swatches } from '../color';
	import { t } from '../i18n';

	let { value = $bindable() }: { value: string } = $props();

	const choices = swatches();
</script>

<div role="group" aria-label={t('color_group')} class="flex flex-wrap items-center gap-3">
	{#each choices as { hex, name } (hex)}
		<button
			type="button"
			aria-label={t('color_swatch', { name: t(name) })}
			aria-pressed={value.toLowerCase() === hex}
			onclick={() => {
				value = hex;
			}}
			style:background-color={hex}
			class="size-11 rounded-pill border-2 {value.toLowerCase() === hex
				? 'border-ink'
				: 'border-glass-edge'}"
		></button>
	{/each}
	<!-- A native color input always yields #rrggbb. -->
	<input
		type="color"
		bind:value
		aria-label={t('color_custom')}
		title={t('color_custom')}
		class="size-11 cursor-pointer rounded-pill border-2 border-glass-edge bg-glass-strong p-0.5"
	/>
</div>
