/** The accent before setup and when settings have none (the mockup's rose-500). Mirrors `--accent`
 *  in app.css. */
export const DEFAULT_ACCENT = '#c76b85';

/** Relative luminance (WCAG 2.x) of `--ink` (app.css). A test keeps the two in sync, so
 *  no second copy of the color lives in code. */
export const INK_LUMINANCE = 0.041973526925469073;

const HEX = /^#[0-9a-fA-F]{6}$/;

/** True for `#rrggbb` only: no short form, no alpha, no names. */
export function isHexColor(s: string): boolean {
	return HEX.test(s);
}

const channel = (v: number): number => {
	const c = v / 255;
	return c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
};

/** WCAG relative luminance of a `#rrggbb` color, 0 (black) to 1 (white). */
export function luminance(hex: string): number {
	const n = Number.parseInt(hex.slice(1), 16);
	return (
		0.2126 * channel((n >> 16) & 0xff) +
		0.7152 * channel((n >> 8) & 0xff) +
		0.0722 * channel(n & 0xff)
	);
}

const contrast = (a: number, b: number): number =>
	(Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05);

/** The text color for a filled accent button: `light` is white, `dark` is `ink`; whichever has the
 *  higher WCAG contrast against the accent, so a pale accent she picks stays readable. */
export function onAccent(hex: string): 'light' | 'dark' {
	const l = luminance(hex);
	return contrast(1, l) >= contrast(INK_LUMINANCE, l) ? 'light' : 'dark';
}

/** Sets `--accent` and `--on-accent` on `root`; null or an invalid color gives the default. */
export function applyAccent(root: HTMLElement, hex: string | null): void {
	const accent = hex !== null && isHexColor(hex) ? hex : DEFAULT_ACCENT;
	root.style.setProperty('--accent', accent);
	root.style.setProperty(
		'--on-accent',
		onAccent(accent) === 'light' ? 'var(--on-strong)' : 'var(--ink)'
	);
}
