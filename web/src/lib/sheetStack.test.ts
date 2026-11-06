import { describe, expect, it, vi } from 'vitest';
import { createSheetStack } from './sheetStack';

function setup() {
	const lock = vi.fn();
	const unlock = vi.fn();
	const showTop = vi.fn<(top: string | null) => void>();
	return { stack: createSheetStack({ lock, unlock, showTop }), lock, unlock, showTop };
}

describe('createSheetStack', () => {
	it('locks the page when the first sheet opens', () => {
		const { stack, lock } = setup();
		stack.push('a');
		expect(lock).toHaveBeenCalledTimes(1);
	});
	it('does not lock again for a second sheet', () => {
		const { stack, lock } = setup();
		stack.push('a');
		stack.push('b');
		expect(lock).toHaveBeenCalledTimes(1);
	});
	it('unlocks only when the last sheet closes', () => {
		const { stack, unlock } = setup();
		stack.push('a');
		stack.push('b');
		stack.remove('b');
		expect(unlock).not.toHaveBeenCalled();
		stack.remove('a');
		expect(unlock).toHaveBeenCalledTimes(1);
	});
	it('unlocks once when sheets close out of order', () => {
		const { stack, unlock } = setup();
		stack.push('a');
		stack.push('b');
		stack.remove('a');
		stack.remove('b');
		expect(unlock).toHaveBeenCalledTimes(1);
	});
	it('says only the newest sheet is on top', () => {
		const { stack } = setup();
		stack.push('a');
		stack.push('b');
		expect([stack.isTop('a'), stack.isTop('b')]).toEqual([false, true]);
	});
	it('gives the top back to the sheet below when the top closes', () => {
		const { stack } = setup();
		stack.push('a');
		stack.push('b');
		stack.remove('b');
		expect(stack.isTop('a')).toBe(true);
	});
	it('keeps the top when a lower sheet closes first', () => {
		const { stack } = setup();
		stack.push('a');
		stack.push('b');
		stack.remove('a');
		expect(stack.isTop('b')).toBe(true);
	});
	it('ignores a sheet that is not open', () => {
		const { stack, unlock } = setup();
		stack.remove('x');
		expect([stack.isTop('x'), unlock.mock.calls.length]).toEqual([false, 0]);
	});
	it('locks again after everything closed', () => {
		const { stack, lock } = setup();
		stack.push('a');
		stack.remove('a');
		stack.push('b');
		expect(lock).toHaveBeenCalledTimes(2);
	});
	it('shows each new top, the one below when the top closes, and null at the end', () => {
		const { stack, showTop } = setup();
		stack.push('a');
		stack.push('b');
		stack.remove('b');
		stack.remove('a');
		expect(showTop.mock.calls).toEqual([['a'], ['b'], ['a'], [null]]);
	});
	it('keeps the top shown when a lower sheet closes first', () => {
		const { stack, showTop } = setup();
		stack.push('a');
		stack.push('b');
		stack.remove('a');
		expect(showTop).toHaveBeenLastCalledWith('b');
	});
});
