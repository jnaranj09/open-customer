<script lang="ts">
	import type { Snippet } from 'svelte';
	import { untrack } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import '../app.css';
	import { api } from '../lib/api/app';
	import type { Me } from '../lib/api/schemas';
	import { errorText } from '../lib/api/errors';
	import { cache } from '../lib/cache/store.svelte';
	import { shouldRefreshOnFocus } from '../lib/cache/refresh';
	import { applyAccent } from '../lib/color';
	import OfflineBanner from '../lib/components/OfflineBanner.svelte';
	import Sidebar from '../lib/components/Sidebar.svelte';
	import TabBar from '../lib/components/TabBar.svelte';
	import UpdatePrompt from '../lib/components/UpdatePrompt.svelte';
	import { canSync, guard, hasShell, needsRecheck, type Session } from '../lib/guard';
	import { t } from '../lib/i18n';
	import { layout } from '../lib/layout';
	import { syncLine } from '../lib/nav';
	import { activatePush, checkPush, type PushState } from '../lib/push';
	import { browserPushDeps } from '../lib/push-browser';
	import { resolveSession } from '../lib/session';
	import { submitOnce } from '../lib/submit';
	import { systemNow } from '../lib/time';

	let { children }: { children: Snippet } = $props();

	const path = $derived(page.url.pathname);

	// What the server said about her, and the path that answer was checked for. The page renders
	// only when both match the path, so a stale answer never shows (or redirects) a new page.
	let session = $state<Session | null>(null);
	let checkedFor = $state<string | null>(null);
	let cacheLoaded = false;
	let settleRun = 0;

	const verdict = $derived(session !== null && checkedFor === path ? guard(path, session) : null);

	async function rememberMe(me: Me): Promise<void> {
		try {
			await cache.rememberMe(me);
		} catch {
			console.warn('code=cache_write_failed');
		}
	}

	async function settle(forPath: string): Promise<void> {
		const run = ++settleRun;
		if (!cacheLoaded) {
			try {
				await cache.load();
			} catch {
				console.warn('code=cache_load_failed');
			}
			cacheLoaded = true;
		}
		if (needsRecheck(session, forPath)) {
			const next = await resolveSession(api, cache.meta.me);
			if (run !== settleRun) {
				return;
			}
			session = next;
			if (next.kind === 'online' && next.me !== null) {
				await rememberMe(next.me);
			}
		}
		checkedFor = forPath;
	}

	// Syncs with the server: each page change asks the guard's question again where it can change.
	$effect(() => {
		const forPath = path;
		untrack(() => {
			void settle(forPath);
		});
	});

	$effect(() => {
		if (verdict?.kind === 'redirect') {
			void goto(resolve(verdict.to), { replaceState: true });
		}
	});

	const syncing = $derived(canSync(session));

	// On open: refresh the cache, then check push. Both again after a new sign-in.
	let pushState = $state<PushState | null>(null);
	let pushError = $state<string | null>(null);
	let pushBusy = $state(false);
	let opened = false;

	$effect(() => {
		if (!syncing) {
			opened = false;
			pushState = null;
			return;
		}
		if (opened) {
			return;
		}
		opened = true;
		void cache.refresh();
		void checkPush(browserPushDeps(api)).then((state) => {
			pushState = state;
		});
	});

	const push = submitOnce(
		async () => {
			pushError = null;
			const result = await activatePush(browserPushDeps(api));
			if (result.ok) {
				pushState = 'active';
			} else {
				pushError = errorText(result.error);
			}
		},
		(pending) => {
			pushBusy = pending;
		}
	);

	// On focus: refresh when the last refresh is old. One at a time: a repeated focus event while
	// this one runs is dropped, and the store orders it against the refresh on open.
	let focusBusy = false;
	async function onFocus(): Promise<void> {
		if (focusBusy || document.visibilityState !== 'visible') {
			return;
		}
		const last = cache.meta.lastSyncAt;
		if (!shouldRefreshOnFocus(last === null ? null : Date.parse(last), systemNow().getTime())) {
			return;
		}
		focusBusy = true;
		try {
			await cache.refresh();
		} finally {
			focusBusy = false;
		}
	}

	$effect(() => {
		if (!syncing) {
			return;
		}
		const handler = (): void => {
			void onFocus();
		};
		window.addEventListener('focus', handler);
		document.addEventListener('visibilitychange', handler);
		return () => {
			window.removeEventListener('focus', handler);
			document.removeEventListener('visibilitychange', handler);
		};
	});

	$effect(() => {
		applyAccent(document.documentElement, cache.snapshot?.settings.accent_color ?? null);
	});

	const zone = $derived(
		cache.snapshot?.settings.timezone ?? Intl.DateTimeFormat().resolvedOptions().timeZone
	);
	const failure = $derived(
		cache.lastError !== null && !cache.offline ? errorText(cache.lastError) : null
	);
</script>

{#snippet content()}
	<div class="mx-auto flex w-full max-w-360 flex-col gap-4">
		{#if cache.offline}
			<OfflineBanner lastSyncAt={cache.meta.lastSyncAt} {zone} />
		{/if}
		{#if failure !== null}
			<p role="alert" class="glass-card rounded-lg px-5 py-3 text-base text-error">{failure}</p>
		{/if}
		{@render children()}
	</div>
{/snippet}

<UpdatePrompt />

{#if verdict?.kind === 'offline_blank'}
	<main class="grid min-h-dvh place-items-center p-8">
		<p role="status" class="glass-card rounded-lg px-6 py-4 text-base text-ink">
			{t('error_offline')}
		</p>
	</main>
{:else if verdict?.kind === 'stay'}
	{#if !hasShell(path)}
		{@render children()}
	{:else if layout.current === 'wide'}
		<Sidebar
			name={cache.snapshot?.settings.name ?? ''}
			pathname={path}
			dueCount={cache.snapshot?.due.length ?? 0}
			sync={syncLine(cache.meta.lastSyncAt, zone, cache.offline)}
			{pushState}
			{pushBusy}
			{pushError}
			onactivatepush={() => {
				void push.run();
			}}
		/>
		<main class="ml-64 p-8">{@render content()}</main>
	{:else}
		<div class="pt-safe">
			<main class="px-4 pt-4 pb-32">{@render content()}</main>
		</div>
		<TabBar pathname={path} />
	{/if}
{/if}
