/** Wraps a save action so a second tap while it runs does nothing (a busy button cannot double-submit).
 *  `onPending` is told when the busy state flips, so a component can mirror it in its own state. */
export function submitOnce<A extends unknown[], R>(
	fn: (...a: A) => Promise<R>,
	onPending?: (pending: boolean) => void
): { run: (...a: A) => Promise<R | undefined>; readonly pending: boolean } {
	let pending = false;
	const set = (value: boolean): void => {
		pending = value;
		onPending?.(value);
	};
	return {
		async run(...a: A): Promise<R | undefined> {
			if (pending) {
				return undefined;
			}
			set(true);
			try {
				return await fn(...a);
			} finally {
				set(false);
			}
		},
		get pending(): boolean {
			return pending;
		}
	};
}
