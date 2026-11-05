// The refresh while the server does not answer: the shell calls it while `cache.offline` is true,
// so the "Sin conexión" banner goes away without a reload, a focus or a page change.

/** The wait before the first try; each failed try doubles it. */
const RETRY_FIRST_MS = 2_000;
/** The longest wait between two tries. */
const RETRY_MAX_MS = 60_000;

/** The wait before try number `attempt` (0 first): 2 s, 4 s, 8 s… up to 60 s. */
export function retryDelay(attempt: number): number {
	return Math.min(RETRY_FIRST_MS * 2 ** attempt, RETRY_MAX_MS);
}

/** Calls `tryRefresh` after a growing wait (`retryDelay`) and at once on the `online` event of
 *  `events` (the window), which also starts the waits over. It stops when a try returns true (the
 *  server answered) or the returned function is called. One try runs at a time; a try that throws
 *  logs `code=offline_retry_failed` and counts as a failed one. */
export function retryWhileOffline(
	tryRefresh: () => Promise<boolean>,
	events: Pick<EventTarget, 'addEventListener' | 'removeEventListener'>
): () => void {
	let attempt = 0;
	let timer: ReturnType<typeof setTimeout> | undefined;
	let running = false;
	let stopped = false;

	function schedule(): void {
		if (stopped) {
			return;
		}
		timer = setTimeout(() => void tryNow(), retryDelay(attempt));
		attempt += 1;
	}

	async function tryNow(): Promise<void> {
		if (stopped || running) {
			return;
		}
		clearTimeout(timer);
		timer = undefined;
		running = true;
		let answered = false;
		try {
			answered = await tryRefresh();
		} catch {
			console.warn('code=offline_retry_failed');
		} finally {
			running = false;
		}
		if (!answered) {
			schedule();
		}
	}

	const onOnline = (): void => {
		attempt = 0;
		void tryNow();
	};
	events.addEventListener('online', onOnline);
	schedule();

	return () => {
		stopped = true;
		clearTimeout(timer);
		events.removeEventListener('online', onOnline);
	};
}
