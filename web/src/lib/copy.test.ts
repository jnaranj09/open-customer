import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { COPIED_MS, copiedFlag, copyText } from './copy';

describe('copyText', () => {
	afterEach(() => {
		vi.unstubAllGlobals();
		vi.restoreAllMocks();
	});

	const stubClipboard = (clipboard: unknown): void => {
		vi.stubGlobal('navigator', { clipboard });
		vi.spyOn(console, 'warn').mockImplementation(() => undefined);
	};

	it('writes the text and reports done', async () => {
		const writeText = vi.fn(() => Promise.resolve());
		stubClipboard({ writeText });
		const done = vi.fn();
		const failed = vi.fn();
		copyText('Hola', done, failed);
		await vi.waitFor(() => {
			expect(done).toHaveBeenCalled();
		});
		expect([writeText.mock.calls, failed.mock.calls]).toEqual([[['Hola']], []]);
	});

	it.each([
		['a missing clipboard', undefined],
		['a refused write', { writeText: () => Promise.reject(new Error('denied')) }],
		[
			'a write that throws',
			{
				writeText: () => {
					throw new Error('no');
				}
			}
		]
	])('reports a failure and logs its code for %s', async (_, clipboard) => {
		stubClipboard(clipboard);
		const failed = vi.fn();
		copyText('Hola', vi.fn(), failed);
		await vi.waitFor(() => {
			expect(failed).toHaveBeenCalled();
		});
		expect(console.warn).toHaveBeenCalledWith('code=clipboard_failed');
	});
});

describe('copiedFlag', () => {
	beforeEach(() => {
		vi.useFakeTimers();
	});
	afterEach(() => {
		vi.useRealTimers();
	});

	it('clears the flag COPIED_MS after the last copy', () => {
		const states: boolean[] = [];
		const flag = copiedFlag((on) => states.push(on));
		flag.show();
		vi.advanceTimersByTime(COPIED_MS - 1);
		flag.show();
		vi.advanceTimersByTime(COPIED_MS);
		expect(states).toEqual([true, true, false]);
	});

	it('stop drops the pending reset', () => {
		const states: boolean[] = [];
		const flag = copiedFlag((on) => states.push(on));
		flag.show();
		flag.stop();
		vi.advanceTimersByTime(COPIED_MS);
		expect(states).toEqual([true]);
	});
});
