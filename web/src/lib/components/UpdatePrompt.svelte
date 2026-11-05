<script lang="ts">
	import { newVersion } from '../api/app';
	import { t } from '../i18n';
	import { submitOnce } from '../submit';
	import { applyUpdate } from '../update';
	import Button from './Button.svelte';

	// `standalone`: a page with no shell (login, setup) gives it its own margins.
	let { standalone = false }: { standalone?: boolean } = $props();
	let busy = $state(false);

	const update = submitOnce(
		async () => {
			const reg = await navigator.serviceWorker.getRegistration();
			if (reg === undefined) {
				location.reload();
				return;
			}
			await applyUpdate(reg, () => {
				location.reload();
			});
		},
		(pending) => {
			busy = pending;
		}
	);
</script>

<!-- In the page's flow, above the offline banner (the layout places it), so neither covers the other. -->
{#if $newVersion}
	<div
		role="status"
		class="glass-card flex items-center gap-3 rounded-lg bg-glass-tint py-3 pr-3 pl-5 lg:ml-auto lg:w-96 {standalone
			? 'm-4'
			: ''}"
	>
		<p class="flex-1 text-base text-ink">{t('update_available')}</p>
		<div class="w-32 shrink-0">
			<Button
				variant="primary"
				{busy}
				onclick={() => {
					void update.run();
				}}
			>
				{t('update_apply')}
			</Button>
		</div>
	</div>
{/if}
