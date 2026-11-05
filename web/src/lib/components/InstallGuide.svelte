<script lang="ts">
	import Card from './ui/Card.svelte';
	import { t, type MessageKey } from '../i18n';
	import Icon from './icons/Icon.svelte';
	import type { IconName } from './icons/paths';

	const steps: readonly { icon: IconName; text: MessageKey }[] = [
		{ icon: 'share', text: 'install_step_share' },
		{ icon: 'square-plus', text: 'install_step_add' },
		{ icon: 'check', text: 'install_step_confirm' }
	];
</script>

<Card as="section" pad="md" class="flex flex-col gap-4">
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
</Card>
