<script lang="ts">
	import type { Snippet } from 'svelte';
	import { untrack } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { version } from '$app/env';
	import { page } from '$app/state';
	import '../app.css';
	import { api, sendErrorReport } from '../lib/api/app';
	import type { Me } from '../lib/api/schemas';
	import { errorText } from '../lib/api/errors';
	import { cache } from '../lib/cache/store.svelte';
	import { shouldRefreshOnFocus } from '../lib/cache/refresh';
	import { retryWhileOffline } from '../lib/cache/retry';
	import { applyAccent } from '../lib/color';
	import ConfirmSentSheet from '../lib/components/ConfirmSentSheet.svelte';
	import OfflineBanner from '../lib/components/ui/OfflineBanner.svelte';
	import Sidebar from '../lib/components/Sidebar.svelte';
	import BottomNav from '../lib/components/ui/BottomNav.svelte';
	import Card from '../lib/components/ui/Card.svelte';
	import UpdatePrompt from '../lib/components/UpdatePrompt.svelte';
	import {
		canSync,
		hasShell,
		needsRecheck,
		recoversSession,
		verdictFor,
		type Session
	} from '../lib/guard';
	import { t } from '../lib/i18n';
	import { layout } from '../lib/layout';
	import { syncLine } from '../lib/nav';
	import { pageTitle } from '../lib/titles';
	import { clock } from '../lib/clock.svelte';
	import { createReporter } from '../lib/report';
	import { pushControl } from '../lib/pushControl.svelte';
	import { resolveSession } from '../lib/session';
	import { finishingStep, sessionStore } from '../lib/setup';
	import { shownDay } from '../lib/shownDay.svelte';
	import { systemNow } from '../lib/time';

	let { children }: { children: Snippet } = $props();

	const path = $derived(page.url.pathname);

	// What the server said about her, and the path that answer was checked for. The page renders
	// only when both match the path, so a stale answer never shows (or redirects) a new page.
	let session = $state<Session | null>(null);
	let checkedFor = $state<string | null>(null);
	let cacheLoaded = false;
	let settleRun = 0;
	let recoverRun = 0;

	// The setup wizard's last steps survive a reload (lib/setup.ts `finishingStep`).
	const verdict = $derived(
		verdictFor(session, checkedFor, path, finishingStep(sessionStore()) !== null)
	);

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

	// The server answered after the session was offline: ask who she is again, so the shell leaves
	// the offline state without a reload.
	async function recover(refreshOk: boolean): Promise<void> {
		if (!recoversSession(session, refreshOk)) {
			return;
		}
		// Its own counter: bumping `settleRun` would strand a page check that is still running.
		const run = ++recoverRun;
		const next = await resolveSession(api, cache.meta.me);
		if (run !== recoverRun) {
			return;
		}
		session = next;
		if (next.kind === 'online' && next.me !== null) {
			await rememberMe(next.me);
		}
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
	let opened = false;

	$effect(() => {
		cache.setSyncing(syncing);
		if (!syncing) {
			opened = false;
			pushControl.reset();
			return;
		}
		if (opened) {
			return;
		}
		opened = true;
		void cache.refresh().then((r) => recover(r.ok));
		void pushControl.check();
	});

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
			await recover((await cache.refresh()).ok);
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

	// While the server does not answer: try again after a growing wait (2 s up to 60 s) and at once
	// when the phone gets a connection back, so the banner clears with the app left open.
	$effect(() => {
		if (!syncing || !cache.offline) {
			return;
		}
		return retryWhileOffline(async () => {
			await recover((await cache.refresh()).ok);
			return !cache.offline;
		}, window);
	});

	// Any uncaught error or rejection goes to the server log, route and stack only (item 66).
	const report = createReporter(sendErrorReport, () => systemNow().getTime());
	$effect(() => {
		const send = (e: unknown): void => {
			// Only with a session: the endpoint needs one, and setup and login have none.
			if (!canSync(session)) {
				return;
			}
			report(e, page.route.id ?? '/', version);
		};
		const onError = (ev: ErrorEvent): void => {
			send(ev.error);
		};
		const onRejection = (ev: PromiseRejectionEvent): void => {
			send(ev.reason);
		};
		window.addEventListener('error', onError);
		window.addEventListener('unhandledrejection', onRejection);
		return () => {
			window.removeEventListener('error', onError);
			window.removeEventListener('unhandledrejection', onRejection);
		};
	});

	// The cached settings own the color; before any cache (login on a new device) the color comes
	// from the setup status answer.
	$effect(() => {
		const publicAccent = session?.kind === 'online' ? session.accent : null;
		applyAccent(document.documentElement, cache.snapshot?.settings.accent_color ?? publicAccent);
	});

	const zone = $derived(
		cache.snapshot?.settings.timezone ?? Intl.DateTimeFormat().resolvedOptions().timeZone
	);
	const failure = $derived(
		cache.lastError !== null && !cache.offline ? errorText(cache.lastError) : null
	);
</script>

<svelte:head>
	<title>{pageTitle(page.route.id)}</title>
</svelte:head>

{#snippet content()}
	<div class="mx-auto flex w-full max-w-360 flex-col gap-4">
		<UpdatePrompt />
		{#if cache.offline}
			<OfflineBanner lastSyncAt={cache.meta.lastSyncAt} {zone} />
		{/if}
		{#if failure !== null}
			<Card as="p" role="alert" pad="message" class="text-base text-error">{failure}</Card>
		{/if}
		{@render children()}
	</div>
{/snippet}

{#if verdict?.kind === 'offline_blank'}
	<main class="grid min-h-dvh place-items-center p-8">
		<Card as="p" role="status" pad="message" class="text-base text-ink">
			{t('error_offline')}
		</Card>
	</main>
{:else if verdict?.kind === 'stay'}
	{#if !hasShell(path)}
		<UpdatePrompt standalone />
		{@render children()}
	{:else if layout.current === 'wide'}
		<Sidebar
			name={cache.snapshot?.settings.name ?? ''}
			customIcon={cache.snapshot?.settings.has_custom_icon}
			pathname={path}
			dueCount={cache.shownDue.length}
			newDay={shownDay.current ?? clock.today(zone)}
			sync={syncLine(cache.meta.lastSyncAt, zone, cache.offline)}
		/>
		<main class="ml-64 p-8">{@render content()}</main>
	{:else}
		<div class="pt-safe">
			<main class="px-4 pt-4 pb-32">{@render content()}</main>
		</div>
		<BottomNav pathname={path} />
	{/if}
{/if}

<!-- Opens first on any app start while a send confirmation is stored (design doc "Sending flow"). -->
{#if verdict?.kind === 'stay' && hasShell(path)}
	<ConfirmSentSheet />
{/if}
