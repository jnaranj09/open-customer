import { describe, expect, it, vi } from 'vitest';
import { submitOnce } from './submit';

function gate() {
	let open!: () => void;
	const wait = new Promise<void>((resolve) => {
		open = resolve;
	});
	return { wait, open };
}

describe('submitOnce', () => {
	it('runs the action once while it is pending', async () => {
		const g = gate();
		const action = vi.fn(() => g.wait);
		const once = submitOnce(action);
		const first = once.run();
		const second = once.run();
		g.open();
		await first;
		expect(await second).toBeUndefined();
		expect(action).toHaveBeenCalledTimes(1);
	});

	it('runs again after the first call settles', async () => {
		const action = vi.fn(() => Promise.resolve('saved'));
		const once = submitOnce(action);
		await once.run();
		expect(await once.run()).toBe('saved');
		expect(action).toHaveBeenCalledTimes(2);
	});

	it('is pending only while the action runs, even when it throws', async () => {
		const g = gate();
		const states: boolean[] = [];
		const once = submitOnce(
			() => g.wait.then(() => Promise.reject(new Error('boom'))),
			(pending) => states.push(pending)
		);
		const running = once.run();
		const during = once.pending;
		g.open();
		await expect(running).rejects.toThrow('boom');
		expect([during, once.pending, states]).toEqual([true, false, [true, false]]);
	});
});
