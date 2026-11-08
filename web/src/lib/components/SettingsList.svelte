<script lang="ts">
	import Card from './ui/Card.svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import type { ResolvedPathname } from '$app/types';
	import { api } from '../api/app';
	import { errorText } from '../api/errors';
	import { clearCache } from '../cache/db';
	import { t, type MessageKey } from '../i18n';
	import { layout } from '../layout';
	import { logOut } from '../logout';
	import { submitOnce } from '../submit';
	import Icon from './icons/Icon.svelte';
	import ListRow from './ui/ListRow.svelte';

	// The Ajustes sections (the mockup's two groups): the screen on compact, the 280 px column on
	// wide, where the open section is marked. "Cerrar sesión" ends the session; the rest are links.
	type Section = { label: MessageKey; href: ResolvedPathname; prefix: string };
	const config: Section[] = [
		{ label: 'settings_business', href: resolve('/ajustes/negocio'), prefix: '/ajustes/negocio' },
		{
			label: 'settings_attributes',
			href: resolve('/ajustes/atributos'),
			prefix: '/ajustes/atributos'
		},
		{
			label: 'settings_templates',
			href: resolve('/ajustes/plantillas'),
			prefix: '/ajustes/plantillas'
		},
		{
			label: 'settings_rules',
			href: resolve('/ajustes/recordatorios'),
			prefix: '/ajustes/recordatorios'
		},
		{ label: 'settings_status', href: resolve('/ajustes/estado'), prefix: '/ajustes/estado' }
	];
	const account: Section[] = [
		{ label: 'settings_export', href: resolve('/ajustes/exportar'), prefix: '/ajustes/exportar' },
		{ label: 'settings_password', href: resolve('/cambiar-clave'), prefix: '/cambiar-clave' }
	];

	// On wide `/ajustes` shows Negocio, so Negocio is the marked row there.
	const path = $derived(page.url.pathname.replace(/\/$/, ''));
	const open = (prefix: string): boolean =>
		layout.current === 'wide' &&
		(path === prefix ||
			path.startsWith(`${prefix}/`) ||
			(path === '/ajustes' && prefix === '/ajustes/negocio'));

	let error = $state<string | null>(null);

	const leave = submitOnce(async () => {
		error = null;
		const result = await logOut({
			logout: () => api.logout(),
			clear: clearCache,
			toLogin: () => goto(resolve('/login'))
		});
		if (!result.ok) {
			error = errorText(result.error);
		}
	});
</script>

{#snippet group(sections: Section[])}
	{#each sections as section, i (section.href)}
		{#if i > 0}
			<li class="mx-3 h-px bg-glass-edge" aria-hidden="true"></li>
		{/if}
		<li>
			<ListRow href={section.href} title={t(section.label)} selected={open(section.prefix)} />
		</li>
	{/each}
{/snippet}

<nav aria-label={t('settings_sections')} class="flex flex-col gap-4">
	<Card as="ul" pad="list" class="flex flex-col">
		{@render group(config)}
	</Card>
	<Card as="ul" pad="list" class="flex flex-col">
		{@render group(account)}
		<li class="mx-3 h-px bg-glass-edge" aria-hidden="true"></li>
		<li>
			<ListRow
				title={t('settings_logout')}
				onclick={() => {
					void leave.run();
				}}
			>
				{#snippet trailing()}
					<Icon name="log-out" class="size-4.5 text-ink-muted" />
				{/snippet}
			</ListRow>
		</li>
	</Card>
	{#if error !== null}
		<p role="alert" class="px-2 text-sm text-error">{error}</p>
	{/if}
</nav>
