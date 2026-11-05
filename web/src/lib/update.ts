/** Runs the update flow of design item 66: check for a new worker, tell the waiting one to take
 *  over, and reload when it controls the page. If no new worker takes over within `timeoutMs`, the
 *  page reloads anyway. Resolves once `reload` has been called (it is called at most once). */
export function applyUpdate(
	reg: ServiceWorkerRegistration,
	reload: () => void,
	timeoutMs = 10_000
): Promise<void> {
	return new Promise((resolve) => {
		const container = navigator.serviceWorker;

		const finish = (): void => {
			clearTimeout(timer);
			container.removeEventListener('controllerchange', finish);
			reload();
			resolve();
		};
		const takeOver = (worker: ServiceWorker): void => {
			worker.postMessage({ type: 'SKIP_WAITING' });
		};

		const timer = setTimeout(finish, timeoutMs);
		container.addEventListener('controllerchange', finish, { once: true });

		void reg.update().then(
			() => {
				if (reg.waiting) {
					takeOver(reg.waiting);
					return;
				}
				const installing = reg.installing;
				if (!installing) {
					return;
				}
				const onState = (): void => {
					if (installing.state === 'installing') {
						return;
					}
					// The install ended (installed or redundant): one answer per update, and a later
					// state change during the reload must not post SKIP_WAITING again.
					installing.removeEventListener('statechange', onState);
					if (installing.state === 'installed' && reg.waiting) {
						takeOver(reg.waiting);
					}
				};
				installing.addEventListener('statechange', onState);
			},
			() => {
				// A failed check changes nothing: the timeout above reloads the page.
			}
		);
	});
}
