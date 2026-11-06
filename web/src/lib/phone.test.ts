import { describe, expect, it } from 'vitest';
import { formatPhone } from './phone';

describe('formatPhone', () => {
	it.each([
		['+50688887777', '+506 8888 7777'],
		['+15551234567', '+1 555 123 4567'],
		['+525512345678', '+52 55 1234 5678'],
		['+34612345678', '+34 612 345 678'],
		['+56912345678', '+56 9 1234 5678'],
		['+593991234567', '+593 99 123 4567']
	])('groups %s by its country as %s', (stored, shown) => {
		expect(formatPhone(stored)).toBe(shown);
	});

	it('shows a number of a country it does not know as stored', () => {
		expect(formatPhone('+4915112345678')).toBe('+4915112345678');
	});

	it('shows a number of an unexpected length as stored', () => {
		expect(formatPhone('+5068888777')).toBe('+5068888777');
	});

	it('shows a value that is not E.164 as stored', () => {
		expect(formatPhone('8888 7777')).toBe('8888 7777');
	});
});
