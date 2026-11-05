import { afterEach, describe, expect, it, vi } from 'vitest';
import {
	DECODE_TIMEOUT_MS,
	fitLongEdge,
	squareCrop,
	toIconPngs,
	toJpegs,
	withTimeout
} from './photos';

afterEach(() => {
	vi.useRealTimers();
	vi.unstubAllGlobals();
});

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

describe('toIconPngs', () => {
	it('rejects with the timeout when the browser never finishes decoding', async () => {
		vi.useFakeTimers();
		vi.stubGlobal('createImageBitmap', () => new Promise<never>(() => undefined));
		const outcome = toIconPngs(new File(['x'], 'icon.png')).catch((e: unknown) =>
			e instanceof Error ? e.message : ''
		);
		await vi.advanceTimersByTimeAsync(DECODE_TIMEOUT_MS);
		expect(await outcome).toBe('timeout');
	});
});

describe('toJpegs', () => {
	type Export = { w: number; h: number; type: string | undefined; quality: number | undefined };

	/** A browser with a 4032x3024 photo and a canvas that records what it was asked to export. */
	function stubBrowser(): Export[] {
		const exports: Export[] = [];
		vi.stubGlobal('createImageBitmap', () =>
			Promise.resolve({ width: 4032, height: 3024, close: () => undefined })
		);
		vi.stubGlobal('document', {
			createElement: () => {
				const canvas = {
					width: 0,
					height: 0,
					getContext: () => ({ drawImage: () => undefined }),
					toBlob: (done: (b: Blob) => void, type?: string, quality?: number) => {
						exports.push({ w: canvas.width, h: canvas.height, type, quality });
						done(new Blob(['x'], { type: type ?? '' }));
					}
				};
				return canvas;
			}
		});
		return exports;
	}

	it('exports the full photo at 2048 px and the thumbnail at 400 px, as JPEG at 0.8', async () => {
		const exports = stubBrowser();
		await toJpegs(new File(['x'], 'photo.heic'));
		expect(exports).toEqual([
			{ w: 2048, h: 1536, type: 'image/jpeg', quality: 0.8 },
			{ w: 400, h: 300, type: 'image/jpeg', quality: 0.8 }
		]);
	});

	it('rejects with the timeout when the browser never finishes decoding', async () => {
		vi.useFakeTimers();
		vi.stubGlobal('createImageBitmap', () => new Promise<never>(() => undefined));
		const outcome = toJpegs(new File(['x'], 'photo.heic')).catch((e: unknown) =>
			e instanceof Error ? e.message : ''
		);
		await vi.advanceTimersByTimeAsync(DECODE_TIMEOUT_MS);
		expect(await outcome).toBe('timeout');
	});
});
