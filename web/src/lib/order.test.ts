import { describe, expect, it } from 'vitest';
import { byPosition } from './order';

describe('byPosition', () => {
	it('sorts by position, then by name', () => {
		const rows = [
			{ position: 1, name: 'b' },
			{ position: 0, name: 'z' },
			{ position: 1, name: 'a' }
		];
		expect(byPosition(rows, (r) => r.name).map((r) => r.name)).toEqual(['z', 'a', 'b']);
	});

	it('does not change the list it is given', () => {
		const rows = [
			{ position: 1, name: 'b' },
			{ position: 0, name: 'a' }
		];
		byPosition(rows, (r) => r.name);
		expect(rows.map((r) => r.name)).toEqual(['b', 'a']);
	});
});
