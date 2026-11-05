<script lang="ts">
	import { newVersion } from '../api/app';
	import { t } from '../i18n';
	import { submitOnce } from '../submit';
	import { applyUpdate } from '../update';
	import Button from './Button.svelte';

	// `standalone`: a page with no shell (login, setup) floats it at the top, so it does not push
	// the centered card down; those pages have no offline banner for it to cover.
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

{#snippet prompt()}
	<div
		role="status"
		class="glass-card flex items-center gap-3 rounded-lg bg-glass-tint py-3 pr-3 pl-5 lg:ml-auto lg:w-96"
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
{/snippet}

<!-- With the shell: in the page's flow, above the offline banner (the layout places it), so neither
     covers the other. -->
{#if $newVersion}
	{#if standalone}
		<div class="fixed inset-x-0 top-0 z-10 px-4 pt-safe">
			<div class="mt-4">{@render prompt()}</div>
		</div>
	{:else}
		{@render prompt()}
	{/if}
{/if}
