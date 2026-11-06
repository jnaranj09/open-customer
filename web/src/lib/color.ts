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

/** WCAG contrast ratio of two relative luminances, 1 to 21. */
export const contrast = (a: number, b: number): number =>
	(Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05);

/** WCAG AA for normal text: what every label on an accent fill must reach. */
export const AA_TEXT = 4.5;

/** How much darker than the accent a filled control is: the mockup fills Button/Strong with
 *  rose-700, the accent a fifth of the way to black. */
const FILL_DARKEN = 0.2;
/** The further step when neither text color reaches AA on that fill. */
const FILL_STEP = 0.05;

/** `hex` moved `by` (0 to 1) of the way to black, channel by channel. */
function darken(hex: string, by: number): string {
	const n = Number.parseInt(hex.slice(1), 16);
	const ch = (shift: number): string =>
		Math.round(((n >> shift) & 0xff) * (1 - by))
			.toString(16)
			.padStart(2, '0');
	return `#${ch(16)}${ch(8)}${ch(0)}`;
}

/** The fill of a filled accent control (strong button, the "+", a selected toggle, a count badge)
 *  and its text color: `light` is white, `dark` is `ink`. The fill is the accent's 700 shade;
 *  the text is whichever of the two has the higher contrast on it. When neither reaches AA (a
 *  mid-tone accent), the fill darkens further until white does, so any accent she picks gives
 *  readable labels. */
export function accentFill(hex: string): { fill: string; text: 'light' | 'dark' } {
	const base = darken(hex, FILL_DARKEN);
	const l = luminance(base);
	const light = contrast(1, l);
	const dark = contrast(INK_LUMINANCE, l);
	if (Math.max(light, dark) >= AA_TEXT) {
		return { fill: base, text: light >= dark ? 'light' : 'dark' };
	}
	let by = FILL_DARKEN;
	let fill = base;
	while (contrast(1, luminance(fill)) < AA_TEXT) {
		by = Math.min(1, by + FILL_STEP);
		fill = darken(hex, by);
	}
	return { fill, text: 'light' };
}

/** Sets `--accent`, `--accent-fill` and `--on-accent` on `root`; null or an invalid color gives
 *  the default. */
export function applyAccent(root: HTMLElement, hex: string | null): void {
	const accent = hex !== null && isHexColor(hex) ? hex : DEFAULT_ACCENT;
	const { fill, text } = accentFill(accent);
	root.style.setProperty('--accent', accent);
	root.style.setProperty('--accent-fill', fill);
	root.style.setProperty('--on-accent', text === 'light' ? 'var(--on-strong)' : 'var(--ink)');
}

const SWATCH_HUES: readonly number[] = [14, 38, 140, 185, 215, 265, 305];

/** `#rrggbb` of an HSL color (h in degrees, s and l in percent). */
function hslToHex(h: number, s: number, l: number): string {
	const sat = s / 100;
	const light = l / 100;
	const a = sat * Math.min(light, 1 - light);
	const channelAt = (n: number): string => {
		const k = (n + h / 30) % 12;
		const v = light - a * Math.max(-1, Math.min(k - 3, 9 - k, 1));
		return Math.round(v * 255)
			.toString(16)
			.padStart(2, '0');
	};
	return `#${channelAt(0)}${channelAt(8)}${channelAt(4)}`;
}

/** The accent choices the color picker offers: the default accent first, then soft hues of
 *  similar weight. Built here because a literal color may not live in a component. */
export function swatches(): string[] {
	return [DEFAULT_ACCENT, ...SWATCH_HUES.map((h) => hslToHex(h, 45, 62))];
}
