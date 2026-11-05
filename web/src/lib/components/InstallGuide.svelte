<script lang="ts">
	import { t, type MessageKey } from '../i18n';
	import Icon, { type IconName } from './icons/Icon.svelte';

	const steps: readonly { icon: IconName; text: MessageKey }[] = [
		{ icon: 'share', text: 'install_step_share' },
		{ icon: 'square-plus', text: 'install_step_add' },
		{ icon: 'check', text: 'install_step_confirm' }
	];
</script>

<section class="glass-card flex flex-col gap-4 rounded-lg p-5">
	<h2 class="text-xl font-bold text-ink">{t('install_title')}</h2>
	<ol class="flex flex-col gap-3">
		{#each steps as step, i (step.text)}
			<li class="flex items-center gap-3">
				<span
					class="flex size-11 shrink-0 items-center justify-center rounded-pill bg-accent-100 text-accent-700"
				>
					<Icon name={step.icon} class="size-5" />
				</span>
				<span class="text-base text-ink"
					><span class="font-bold">{i + 1}.</span> {t(step.text)}</span
				>
			</li>
		{/each}
	</ol>
</section>
