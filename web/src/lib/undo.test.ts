import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { delayed, onPageLeave } from './undo';

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

	it('flush runs the action now, once, and waits for it', async () => {
		const action = vi.fn(() => Promise.resolve());
		const pending = delayed(action, 5000);
		const outcome = await pending.flush();
		await vi.advanceTimersByTimeAsync(10_000);
		expect([outcome, action.mock.calls.length]).toEqual(['ran', 1]);
	});

	it('flush after cancel does not run the action', async () => {
		const action = vi.fn(() => Promise.resolve());
		const pending = delayed(action, 5000);
		pending.cancel();
		expect([await pending.flush(), action.mock.calls.length]).toEqual(['cancelled', 0]);
	});

	it('flush rejects when the action fails', async () => {
		const pending = delayed(() => Promise.reject(new Error('boom')), 5000);
		await expect(pending.flush()).rejects.toEqual(new Error('boom'));
	});
});

/** A document whose visibility the test sets. */
function fakeDocument(): EventTarget & { visibilityState: DocumentVisibilityState } {
	const state: { visibilityState: DocumentVisibilityState } = { visibilityState: 'visible' };
	return Object.assign(new EventTarget(), state);
}

describe('onPageLeave', () => {
	it('calls leave on pagehide', () => {
		const leave = vi.fn();
		const win = new EventTarget();
		onPageLeave(leave, win, fakeDocument());
		win.dispatchEvent(new Event('pagehide'));
		expect(leave).toHaveBeenCalledTimes(1);
	});

	it('calls leave when the page becomes hidden', () => {
		const leave = vi.fn();
		const doc = fakeDocument();
		onPageLeave(leave, new EventTarget(), doc);
		doc.visibilityState = 'hidden';
		doc.dispatchEvent(new Event('visibilitychange'));
		expect(leave).toHaveBeenCalledTimes(1);
	});

	it('does not call leave when the page becomes visible', () => {
		const leave = vi.fn();
		const doc = fakeDocument();
		onPageLeave(leave, new EventTarget(), doc);
		doc.dispatchEvent(new Event('visibilitychange'));
		expect(leave).not.toHaveBeenCalled();
	});

	it('stops listening once removed', () => {
		const leave = vi.fn();
		const win = new EventTarget();
		const doc = fakeDocument();
		onPageLeave(leave, win, doc)();
		doc.visibilityState = 'hidden';
		win.dispatchEvent(new Event('pagehide'));
		doc.dispatchEvent(new Event('visibilitychange'));
		expect(leave).not.toHaveBeenCalled();
	});
});
