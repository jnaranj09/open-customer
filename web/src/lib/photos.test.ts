import { describe, expect, it, vi } from 'vitest';
import { DECODE_TIMEOUT_MS, fitLongEdge, squareCrop, withTimeout } from './photos';

describe('fitLongEdge', () => {
	it('scales 4032x3024 to 2048x1536', () => {
		expect(fitLongEdge(4032, 3024, 2048)).toEqual({ w: 2048, h: 1536 });
	});

	it('scales a portrait photo on its height', () => {
		expect(fitLongEdge(3024, 4032, 400)).toEqual({ w: 300, h: 400 });
	});

	it('never upscales', () => {
		expect(fitLongEdge(300, 200, 2048)).toEqual({ w: 300, h: 200 });
	});

	it('keeps at least one pixel on the short side', () => {
		expect(fitLongEdge(10000, 1, 400)).toEqual({ w: 400, h: 1 });
	});
});

describe('squareCrop', () => {
	it('centers a landscape image', () => {
		expect(squareCrop(400, 200)).toEqual({ sx: 100, sy: 0, size: 200 });
	});

	it('centers a portrait image', () => {
		expect(squareCrop(200, 400)).toEqual({ sx: 0, sy: 100, size: 200 });
	});

	it('keeps a square as is', () => {
		expect(squareCrop(300, 300)).toEqual({ sx: 0, sy: 0, size: 300 });
	});
});

describe('withTimeout', () => {
	it('rejects when the work never settles', async () => {
		vi.useFakeTimers();
		const result = withTimeout(new Promise<never>(() => undefined), DECODE_TIMEOUT_MS);
		const outcome = result.catch((e: unknown) => (e instanceof Error ? e.message : ''));
		await vi.advanceTimersByTimeAsync(DECODE_TIMEOUT_MS);
		expect(await outcome).toBe('timeout');
		vi.useRealTimers();
	});

	it('gives the value when the work settles first', async () => {
		expect(await withTimeout(Promise.resolve(7), DECODE_TIMEOUT_MS)).toBe(7);
	});

	it('passes the work error through', async () => {
		await expect(withTimeout(Promise.reject(new Error('bad')), DECODE_TIMEOUT_MS)).rejects.toThrow(
			'bad'
		);
	});
});
