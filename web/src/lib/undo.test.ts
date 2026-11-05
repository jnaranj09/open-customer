import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { delayed } from './undo';

beforeEach(() => {
	vi.useFakeTimers();
});

afterEach(() => {
	vi.useRealTimers();
});

describe('delayed', () => {
	it('runs the action after 5 s', async () => {
		const action = vi.fn(() => Promise.resolve());
		const pending = delayed(action, 5000);
		await vi.advanceTimersByTimeAsync(4999);
		expect(action).not.toHaveBeenCalled();
		await vi.advanceTimersByTimeAsync(1);
		expect(action).toHaveBeenCalledTimes(1);
		expect(await pending.done).toBe('ran');
	});

	it('cancel stops the action', async () => {
		const action = vi.fn(() => Promise.resolve());
		const pending = delayed(action, 5000);
		pending.cancel();
		await vi.advanceTimersByTimeAsync(10_000);
		expect(action).not.toHaveBeenCalled();
		expect(await pending.done).toBe('cancelled');
	});

	it('cancel after the action started does not undo it', async () => {
		const action = vi.fn(() => Promise.resolve());
		const pending = delayed(action, 5000);
		await vi.advanceTimersByTimeAsync(5000);
		pending.cancel();
		expect(await pending.done).toBe('ran');
	});

	it('rejects done when the action fails', async () => {
		const pending = delayed(() => Promise.reject(new Error('boom')), 5000);
		const settled = pending.done.then(
			() => 'ran',
			(error: unknown) => error
		);
		await vi.advanceTimersByTimeAsync(5000);
		expect(await settled).toEqual(new Error('boom'));
	});
});
