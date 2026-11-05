// Photo and icon preparation on the phone (design doc items 69 and 73). The sizing math is pure
// and tested; the canvas parts need a browser and are checked in the running app. `toJpegs` (the
// photo pair, 2048 and 400 px at quality 0.8) comes with the photo screen (Task 11).

export type Size = { w: number; h: number };

/** `w` x `h` scaled so the long edge is `max`; a smaller image keeps its size (never upscales). */
export function fitLongEdge(w: number, h: number, max: number): Size {
	const long = Math.max(w, h);
	if (long <= max) {
		return { w, h };
	}
	const scale = max / long;
	return { w: Math.max(1, Math.round(w * scale)), h: Math.max(1, Math.round(h * scale)) };
}

/** The centered square of a `w` x `h` image. */
export function squareCrop(w: number, h: number): { sx: number; sy: number; size: number } {
	const size = Math.min(w, h);
	return { sx: Math.round((w - size) / 2), sy: Math.round((h - size) / 2), size };
}

/** How long a decode may take before it counts as failed. A browser can leave `createImageBitmap`
 *  or `img.decode()` pending on a file it cannot read; the screen must still end in a message. */
export const DECODE_TIMEOUT_MS = 15_000;

/** `work`, or a rejection once `ms` have passed. */
export function withTimeout<T>(work: Promise<T>, ms: number): Promise<T> {
	return new Promise<T>((resolve, reject) => {
		const timer = setTimeout(() => {
			reject(new Error('timeout'));
		}, ms);
		work.then(
			(value) => {
				clearTimeout(timer);
				resolve(value);
			},
			(error: unknown) => {
				clearTimeout(timer);
				reject(error instanceof Error ? error : new Error('decode failed'));
			}
		);
	});
}

const ICON_BIG = 512;
const ICON_SMALL = 180;

/** A decoded image with its pixel size. */
type Decoded = { source: CanvasImageSource; w: number; h: number; release: () => void };

async function viaImageElement(file: File): Promise<Decoded> {
	const url = URL.createObjectURL(file);
	const img = new Image();
	try {
		img.src = url;
		await img.decode();
	} catch (e) {
		URL.revokeObjectURL(url);
		throw e;
	}
	return {
		source: img,
		w: img.naturalWidth,
		h: img.naturalHeight,
		release: () => {
			URL.revokeObjectURL(url);
		}
	};
}

/** Decodes with the EXIF rotation applied. Falls back to an <img> when `createImageBitmap` or its
 *  `imageOrientation` option is not supported (the <img> element applies the orientation itself). */
async function decode(file: File): Promise<Decoded> {
	if (typeof createImageBitmap === 'function') {
		try {
			const bitmap = await createImageBitmap(file, { imageOrientation: 'from-image' });
			return {
				source: bitmap,
				w: bitmap.width,
				h: bitmap.height,
				release: () => {
					bitmap.close();
				}
			};
		} catch {
			console.warn('code=bitmap_decode_failed');
		}
	}
	return viaImageElement(file);
}

function canvasOf(
	w: number,
	h: number
): { canvas: HTMLCanvasElement; ctx: CanvasRenderingContext2D } {
	const canvas = document.createElement('canvas');
	canvas.width = w;
	canvas.height = h;
	const ctx = canvas.getContext('2d');
	if (ctx === null) {
		throw new Error('no 2d canvas');
	}
	ctx.imageSmoothingQuality = 'high';
	return { canvas, ctx };
}

function toBlob(canvas: HTMLCanvasElement, type: string): Promise<Blob> {
	return new Promise((resolve, reject) => {
		canvas.toBlob((blob) => {
			if (blob === null) {
				reject(new Error('canvas export failed'));
			} else {
				resolve(blob);
			}
		}, type);
	});
}

async function pngSquare(d: Decoded, edge: number): Promise<Blob> {
	const { sx, sy, size } = squareCrop(d.w, d.h);
	const { canvas, ctx } = canvasOf(edge, edge);
	ctx.drawImage(d.source, sx, sy, size, size, 0, 0, edge, edge);
	return toBlob(canvas, 'image/png');
}

/** The app icon pair: the centered square of the image as 512 px and 180 px PNGs. */
export async function toIconPngs(file: File): Promise<{ big: Blob; small: Blob }> {
	const d = await withTimeout(decode(file), DECODE_TIMEOUT_MS);
	try {
		return { big: await pngSquare(d, ICON_BIG), small: await pngSquare(d, ICON_SMALL) };
	} finally {
		d.release();
	}
}
