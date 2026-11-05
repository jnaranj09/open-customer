/** Runs `action` after `ms` unless `cancel` is called first (the Omitir toast, "Deshacer"). `done`
 *  says which happened; if the action ran and failed, `done` rejects with its error. Cancelling
 *  after the action started changes nothing. */
export function delayed(
	action: () => Promise<void>,
	ms: number
): { cancel(): void; done: Promise<'ran' | 'cancelled'> } {
	let timer: ReturnType<typeof setTimeout> | undefined;
	let settle!: (outcome: 'cancelled') => void;
	const done = new Promise<'ran' | 'cancelled'>((resolve, reject) => {
		settle = resolve;
		timer = setTimeout(() => {
			timer = undefined;
			action().then(() => {
				resolve('ran');
			}, reject);
		}, ms);
	});
	return {
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
