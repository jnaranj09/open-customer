import { describe, expect, it } from 'vitest';
import { nextFocus } from './focus';

describe('nextFocus', () => {
	it('moves forward and wraps from the last to the first', () => {
		expect([nextFocus(0, 3, false), nextFocus(1, 3, false), nextFocus(2, 3, false)]).toEqual([
			1, 2, 0
		]);
	});

	it('moves back and wraps from the first to the last', () => {
		expect([nextFocus(2, 3, true), nextFocus(1, 3, true), nextFocus(0, 3, true)]).toEqual([
			1, 0, 2
		]);
	});

	it('enters the trap at the first element going forward and the last going back', () => {
		expect([nextFocus(-1, 3, false), nextFocus(-1, 3, true)]).toEqual([0, 2]);
	});

	it('has nowhere to go with no focusable element', () => {
		expect(nextFocus(-1, 0, false)).toBe(-1);
	});
});
