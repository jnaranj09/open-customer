// The business day on screen: a reactive reading of the clock that only moves when `tick` runs.
// A screen ticks it when the app resumes or refreshes, so a PWA left open overnight stops calling
// yesterday "Hoy".
import { systemNow, todayIn } from './time';

export function createClock(now: () => Date) {
	let current = $state(now());
	return {
		/** The business day (`YYYY-MM-DD`) in `zone` at the last tick. */
		today(zone: string): string {
			return todayIn(zone, current);
		},
		/** Reads the clock again. */
		tick(): void {
			current = now();
		}
	};
}

export const clock = createClock(systemNow);
