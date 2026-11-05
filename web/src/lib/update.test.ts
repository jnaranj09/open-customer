import { afterEach, describe, expect, it, vi } from 'vitest';
import { applyUpdate } from './update';

function fakeContainer() {
	const listeners = new Map<string, () => void>();
	return {
		addEventListener: vi.fn((type: string, fn: () => void) => listeners.set(type, fn)),
		removeEventListener: vi.fn((type: string) => listeners.delete(type)),
		fire: (type: string) => listeners.get(type)?.()
	};
}

function fakeReg(waiting: { postMessage: () => void } | null) {
	return {
		update: vi.fn(() => Promise.resolve()),
		waiting,
		installing: null
	} as unknown as ServiceWorkerRegistration;
}

afterEach(() => {
	vi.useRealTimers();
	vi.unstubAllGlobals();
});

describe('applyUpdate', () => {
	it('reloads after 10 s when no worker shows up', async () => {
		vi.useFakeTimers();
		vi.stubGlobal('navigator', { serviceWorker: fakeContainer() });
		const reload = vi.fn();
		const done = applyUpdate(fakeReg(null), reload);
		await vi.advanceTimersByTimeAsync(9_999);
		expect(reload).not.toHaveBeenCalled();
		await vi.advanceTimersByTimeAsync(1);
		await done;
		expect(reload).toHaveBeenCalledTimes(1);
	});

	it('tells a waiting worker to take over and reloads once on controllerchange', async () => {
		vi.useFakeTimers();
		const container = fakeContainer();
		vi.stubGlobal('navigator', { serviceWorker: container });
		const postMessage = vi.fn();
		const reload = vi.fn();
		const done = applyUpdate(fakeReg({ postMessage }), reload);
		await vi.advanceTimersByTimeAsync(0);
		container.fire('controllerchange');
		await done;
		await vi.advanceTimersByTimeAsync(20_000);
		expect(postMessage).toHaveBeenCalledWith({ type: 'SKIP_WAITING' });
		expect(reload).toHaveBeenCalledTimes(1);
	});
});
