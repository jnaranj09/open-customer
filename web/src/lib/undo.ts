/** How long the "Deshacer" toast stays and how long `delayed` waits: one number, so the toast
 *  never goes away before the action runs, nor stays after. */
export const UNDO_MS = 5000;

/** Runs `action` after `ms` unless `cancel` is called first (the Omitir toast, "Deshacer"). `done`
 *  says which happened; if the action ran and failed, `done` rejects with its error. Cancelling
 *  after the action started changes nothing. `flush` runs the action now, if it has not started,
 *  and returns `done`: used before the page is left, which would otherwise lose the action. */
export function delayed(
	action: () => Promise<void>,
	ms: number
): { cancel(): void; flush(): Promise<'ran' | 'cancelled'>; done: Promise<'ran' | 'cancelled'> } {
	let timer: ReturnType<typeof setTimeout> | undefined;
	let settle!: (outcome: 'cancelled') => void;
	let start!: () => void;
	const done = new Promise<'ran' | 'cancelled'>((resolve, reject) => {
		settle = resolve;
		start = () => {
			timer = undefined;
			action().then(() => {
				resolve('ran');
			}, reject);
		};
		timer = setTimeout(start, ms);
	});
	return {
		flush(): Promise<'ran' | 'cancelled'> {
			if (timer !== undefined) {
				clearTimeout(timer);
				start();
			}
			return done;
		},
		cancel(): void {
			if (timer === undefined) {
				return;
			}
			clearTimeout(timer);
			timer = undefined;
			settle('cancelled');
		},
		done
	};
}
