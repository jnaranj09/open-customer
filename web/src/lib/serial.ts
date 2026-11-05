/** Wraps an async task so two runs never overlap, and a run never ends after one that started
 *  later (an older answer must not overwrite a newer one). A call made while a run is in flight
 *  waits for it, then starts one more run; every call made meanwhile shares that one follow-up,
 *  which starts after those calls, so it sees whatever they were waiting for. */
export function serialLatest<T>(task: () => Promise<T>): () => Promise<T> {
	let running: Promise<T> | null = null;
	let next: Promise<T> | null = null;

	const start = (): Promise<T> => {
		const p = task();
		running = p;
		const clear = (): void => {
			if (running === p && next === null) {
				running = null;
			}
		};
		p.then(clear, clear);
		return p;
	};

	return () => {
		if (running === null) {
			return start();
		}
		next ??= running
			.catch(() => undefined)
			.then(() => {
				next = null;
				return start();
			});
		return next;
	};
}
