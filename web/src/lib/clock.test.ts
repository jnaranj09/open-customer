import { describe, expect, it } from 'vitest';
import { createClock } from './clock.svelte';

const ZONE = 'America/Costa_Rica';

function setup(start: string) {
	let now = new Date(start);
	const clock = createClock(() => now);
	return {
		clock,
		set: (iso: string): void => {
			now = new Date(iso);
		}
	};
}

describe('the day clock', () => {
	it('is the business day at creation', () => {
		const { clock } = setup('2026-11-04T20:00:00Z');
		expect(clock.today(ZONE)).toBe('2026-11-04');
	});

	it('keeps yesterday until it is ticked', () => {
		const { clock, set } = setup('2026-11-04T20:00:00Z');
		set('2026-11-05T14:00:00Z');
		expect(clock.today(ZONE)).toBe('2026-11-04');
	});

	it('moves Hoy to the new day when a resume after midnight ticks it', () => {
		const { clock, set } = setup('2026-11-04T20:00:00Z');
		set('2026-11-05T14:00:00Z');
		clock.tick();
		expect(clock.today(ZONE)).toBe('2026-11-05');
	});

	it('uses the business zone for the midnight', () => {
		const { clock } = setup('2026-11-05T04:30:00Z');
		expect(clock.today(ZONE)).toBe('2026-11-04');
	});
});
