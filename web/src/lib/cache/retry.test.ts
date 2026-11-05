import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { retryDelay, retryWhileOffline } from './retry';

beforeEach(() => {
	vi.useFakeTimers();
});

afterEach(() => {
	vi.useRealTimers();
});

/** A refresh that never reaches the server, counting its calls. */
function offlineRefresh() {
	return vi.fn<() => Promise<boolean>>(() => Promise.resolve(false));
}

/** A refresh whose try stays open until `answer` settles it. */
function pendingRefresh() {
	let settle: (answered: boolean) => void = () => undefined;
	const tryRefresh = vi.fn<() => Promise<boolean>>(
		() =>
			new Promise((resolve) => {
				settle = resolve;
			})
	);
	return {
		tryRefresh,
		answer: (answered: boolean): void => {
			settle(answered);
		}
	};
}

describe('retryDelay', () => {
	it('starts at 2 s and doubles', () => {
		expect([0, 1, 2, 3].map(retryDelay)).toEqual([2_000, 4_000, 8_000, 16_000]);
	});

	it('never waits more than 60 s', () => {
		expect([5, 6, 20].map(retryDelay)).toEqual([60_000, 60_000, 60_000]);
	});
});

describe('retryWhileOffline', () => {
	it('tries after 2 s, then waits longer after each failed try', async () => {
		const tryRefresh = offlineRefresh();
		retryWhileOffline(tryRefresh, new EventTarget());

		await vi.advanceTimersByTimeAsync(1_999);
		expect(tryRefresh).toHaveBeenCalledTimes(0);
		await vi.advanceTimersByTimeAsync(1);
		expect(tryRefresh).toHaveBeenCalledTimes(1);
		await vi.advanceTimersByTimeAsync(3_999);
		expect(tryRefresh).toHaveBeenCalledTimes(1);
		await vi.advanceTimersByTimeAsync(1);
		expect(tryRefresh).toHaveBeenCalledTimes(2);
	});

	it('stops once a try reaches the server', async () => {
		const tryRefresh = vi.fn<() => Promise<boolean>>(() => Promise.resolve(true));
		retryWhileOffline(tryRefresh, new EventTarget());

		await vi.advanceTimersByTimeAsync(10 * 60_000);
		expect(tryRefresh).toHaveBeenCalledTimes(1);
	});

	it('tries at once on the online event and starts the waits over', async () => {
		const tryRefresh = offlineRefresh();
		const events = new EventTarget();
		retryWhileOffline(tryRefresh, events);
		// Two failed tries: the next wait would be 8 s.
		await vi.advanceTimersByTimeAsync(6_000);
		expect(tryRefresh).toHaveBeenCalledTimes(2);

		events.dispatchEvent(new Event('online'));
		await vi.advanceTimersByTimeAsync(0);
		expect(tryRefresh).toHaveBeenCalledTimes(3);
		await vi.advanceTimersByTimeAsync(2_000);
		expect(tryRefresh).toHaveBeenCalledTimes(4);
	});

	it('runs one try at a time when online fires during a try', async () => {
		const { tryRefresh, answer } = pendingRefresh();
		const events = new EventTarget();
		retryWhileOffline(tryRefresh, events);
		await vi.advanceTimersByTimeAsync(2_000);

		events.dispatchEvent(new Event('online'));
		await vi.advanceTimersByTimeAsync(0);
		expect(tryRefresh).toHaveBeenCalledTimes(1);
		answer(true);
	});

	it('counts a try that throws as failed and logs a code', async () => {
		const warn = vi.spyOn(console, 'warn').mockImplementation(() => undefined);
		const tryRefresh = vi
			.fn<() => Promise<boolean>>()
			.mockRejectedValueOnce(new Error('boom'))
			.mockResolvedValue(true);
		retryWhileOffline(tryRefresh, new EventTarget());

		await vi.advanceTimersByTimeAsync(2_000 + 4_000);
		expect(tryRefresh).toHaveBeenCalledTimes(2);
		expect(warn).toHaveBeenCalledWith('code=offline_retry_failed');
	});

	it('leaves no timer after a stop that came while a try ran', async () => {
		const { tryRefresh, answer } = pendingRefresh();
		const stop = retryWhileOffline(tryRefresh, new EventTarget());
		await vi.advanceTimersByTimeAsync(2_000);
		stop();
		answer(false);
		await vi.advanceTimersByTimeAsync(0);

		expect(vi.getTimerCount()).toBe(0);
	});

	it('does nothing more after stop, neither on a timer nor on online', async () => {
		const tryRefresh = offlineRefresh();
		const events = new EventTarget();
		const stop = retryWhileOffline(tryRefresh, events);
		stop();

		events.dispatchEvent(new Event('online'));
		await vi.advanceTimersByTimeAsync(10 * 60_000);
		expect(tryRefresh).toHaveBeenCalledTimes(0);
	});
});
