<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { api } from '../../lib/api/app';
	import { errorText } from '../../lib/api/errors';
	import { cache } from '../../lib/cache/store.svelte';
	import AuthCard from '../../lib/components/AuthCard.svelte';
	import Button from '../../lib/components/ui/Button.svelte';
	import TextField from '../../lib/components/ui/TextField.svelte';
	import { t } from '../../lib/i18n';
	import { submitOnce } from '../../lib/submit';

	let username = $state('');
	let password = $state('');
	let error = $state<string | null>(null);
	let busy = $state(false);

	const submit = submitOnce(
		async () => {
			error = null;
			const result = await api.login({ username, password });
			if (result.ok) {
				// The layout asks the server again on the new path and shows the app.
				await goto(resolve('/'));
			} else {
				error = errorText(result.error);
			}
		},
		(pending) => {
			busy = pending;
		}
	);
</script>

<AuthCard
	customIcon={cache.snapshot?.settings.has_custom_icon}
	title={cache.snapshot?.settings.name ?? t('login_heading')}
	caption={t('login_caption')}
>
	<form
		class="flex flex-col gap-4"
		onsubmit={(e) => {
			e.preventDefault();
			void submit.run();
		}}
	>
		<TextField
			label={t('login_username')}
			bind:value={username}
			autocomplete="username"
			autocapitalize="off"
			autocorrect="off"
		/>
		<TextField
			label={t('login_password')}
			type="password"
			bind:value={password}
			autocomplete="current-password"
		/>
		{#if error !== null}
			<p role="alert" class="text-sm text-error">{error}</p>
		{/if}
		<Button variant="primary" type="submit" {busy}>{t('login_submit')}</Button>
	</form>
</AuthCard>
