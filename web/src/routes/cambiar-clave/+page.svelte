<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { api } from '../../lib/api/app';
	import { errorText } from '../../lib/api/errors';
	import { cache } from '../../lib/cache/store.svelte';
	import AuthCard from '../../lib/components/AuthCard.svelte';
	import Button from '../../lib/components/ui/Button.svelte';
	import { buttonClasses } from '../../lib/components/ui/buttonStyle';
	import TextField from '../../lib/components/ui/TextField.svelte';
	import { t } from '../../lib/i18n';
	import { submitOnce } from '../../lib/submit';

	let current = $state('');
	let next = $state('');
	let currentError = $state<string | undefined>(undefined);
	let error = $state<string | null>(null);
	let busy = $state(false);

	// Forced after a reset: there is nothing to go back to until the new password is set.
	const forced = $derived(cache.meta.me?.must_change_password !== false);

	const submit = submitOnce(
		async () => {
			currentError = undefined;
			error = null;
			const result = await api.changePassword({ current_password: current, new_password: next });
			if (result.ok) {
				await goto(resolve('/'));
				return;
			}
			// A wrong current password is a validation error on its own field.
			const wrong =
				result.error.kind === 'problem' ? result.error.fields['current_password'] : undefined;
			if (wrong === undefined) {
				error = errorText(result.error);
			} else {
				currentError = wrong;
			}
		},
		(pending) => {
			busy = pending;
		}
	);
</script>

<AuthCard
	customIcon={cache.snapshot?.settings.has_custom_icon}
	title={t('password_title')}
	caption={t('password_caption')}
>
	<form
		class="flex flex-col gap-4"
		onsubmit={(e) => {
			e.preventDefault();
			void submit.run();
		}}
	>
		<TextField
			label={t('password_current')}
			type="password"
			bind:value={current}
			autocomplete="current-password"
			error={currentError}
		/>
		<TextField
			label={t('password_new')}
			type="password"
			bind:value={next}
			autocomplete="new-password"
			hint={t('setup_password_hint')}
		/>
		{#if error !== null}
			<p role="alert" class="text-sm text-error">{error}</p>
		{/if}
		<Button variant="strong" type="submit" {busy}>{t('password_save')}</Button>
		{#if !forced}
			<a href={resolve('/ajustes')} class={buttonClasses('glass')}>{t('password_back')}</a>
		{/if}
	</form>
</AuthCard>
