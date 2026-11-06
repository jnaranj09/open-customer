import type { MessageKey } from './i18n';

/** The accent before setup and when settings have none (the mockup's rose-500). Mirrors `--accent`
 *  in app.css. */
export const DEFAULT_ACCENT = '#c76b85';

/** Relative luminance (WCAG 2.x) of `--ink` (app.css). A test keeps the two in sync, so
 *  no second copy of the color lives in code. */
export const INK_LUMINANCE = 0.041973526925469073;

/** Relative luminance of `--bg-100` (app.css), the page background; a test keeps it in sync. */
export const BG_LUMINANCE = 0.9222485011760946;

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

/** `hex` moved `by` (0 to 1) of the way to white, channel by channel. */
function lighten(hex: string, by: number): string {
	const n = Number.parseInt(hex.slice(1), 16);
	const ch = (shift: number): string => {
		const v = (n >> shift) & 0xff;
		return Math.round(v + (255 - v) * by)
			.toString(16)
			.padStart(2, '0');
	};
	return `#${ch(16)}${ch(8)}${ch(0)}`;
}

/** The fill of a filled accent control (strong button, the "+", a selected toggle, a count badge),
 *  its hover fill, and its text color: `light` is white, `dark` is `ink`. The fill is the accent's
 *  700 shade; the text is whichever of the two has the higher contrast on it. When neither reaches
 *  AA (a mid-tone accent), the fill darkens further until white does, so any accent she picks
 *  gives readable labels. The hover fill moves away from the label (darker under white, lighter
 *  under ink), so the label keeps AA on it too. */
export function accentFill(hex: string): { fill: string; hover: string; text: 'light' | 'dark' } {
	const base = darken(hex, FILL_DARKEN);
	const l = luminance(base);
	const light = contrast(1, l);
	const dark = contrast(INK_LUMINANCE, l);
	if (Math.max(light, dark) >= AA_TEXT && dark > light) {
		return { fill: base, hover: lighten(base, HOVER_STEP), text: 'dark' };
	}
	let by = FILL_DARKEN;
	let fill = base;
	while (contrast(1, luminance(fill)) < AA_TEXT) {
		by = Math.min(1, by + FILL_STEP);
		fill = darken(hex, by);
	}
	const darker = darken(hex, Math.min(1, by + HOVER_STEP));
	// A fill already at black cannot get darker: its hover lightens a little, still far past AA.
	return { fill, hover: darker === fill ? lighten(fill, HOVER_STEP) : darker, text: 'light' };
}

/** How far the hover fill moves from the fill. */
const HOVER_STEP = 0.1;

/** sRGB channel (0 to 1) to linear light, and back. */
const toLinear = (c: number): number => (c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4);
const fromLinear = (c: number): number =>
	c <= 0.0031308 ? 12.92 * c : 1.055 * c ** (1 / 2.4) - 0.055;

type Lab = [number, number, number];

/** OKLab of a `#rrggbb` color (Björn Ottosson's matrices, as CSS Color 4 uses them). */
function toOklab(hex: string): Lab {
	const n = Number.parseInt(hex.slice(1), 16);
	const [r, g, b] = [16, 8, 0].map((s) => toLinear(((n >> s) & 0xff) / 255)) as Lab;
	const l = Math.cbrt(0.4122214708 * r + 0.5363325363 * g + 0.0514459929 * b);
	const m = Math.cbrt(0.2119034982 * r + 0.6806995451 * g + 0.1073969566 * b);
	const s = Math.cbrt(0.0883024619 * r + 0.2817188376 * g + 0.6299787005 * b);
	return [
		0.2104542553 * l + 0.793617785 * m - 0.0040720468 * s,
		1.9779984951 * l - 2.428592205 * m + 0.4505937099 * s,
		0.0259040371 * l + 0.7827717662 * m - 0.808675766 * s
	];
}

/** `#rrggbb` of an OKLab color, clipped to sRGB. */
function fromOklab([L, A, B]: Lab): string {
	const l = (L + 0.3963377774 * A + 0.2158037573 * B) ** 3;
	const m = (L - 0.1055613458 * A - 0.0638541728 * B) ** 3;
	const s = (L - 0.0894841775 * A - 1.291485548 * B) ** 3;
	const lin = [
		4.0767416621 * l - 3.3077115913 * m + 0.2309699292 * s,
		-1.2684380046 * l + 2.6097574011 * m - 0.3413193965 * s,
		-0.0041960863 * l - 0.7034186147 * m + 1.707614701 * s
	];
	return `#${lin
		.map((c) =>
			Math.round(Math.min(1, Math.max(0, fromLinear(c))) * 255)
				.toString(16)
				.padStart(2, '0')
		)
		.join('')}`;
}

/** `--accent-100` (app.css: `color-mix(in oklab, var(--accent) 15%, white)`) for `hex`. */
export function accent100(hex: string): string {
	const [L, A, B] = toOklab(hex);
	// White is L 1, a 0, b 0 in OKLab.
	return fromOklab([0.15 * L + 0.85, 0.15 * A, 0.15 * B]);
}

/** The accent as text on a light surface (a ghost button, a text button, a selected chip, a status
 *  chip, the times in the grid): the accent's 700 shade, darkened until it reaches AA on both
 *  the page background (`--bg-100`) and the pale accent tint (`--accent-100`). */
export function accentText(hex: string): string {
	const tint = luminance(accent100(hex));
	let by = FILL_DARKEN;
	let text = darken(hex, by);
	const enough = (c: string): boolean =>
		contrast(BG_LUMINANCE, luminance(c)) >= AA_TEXT && contrast(tint, luminance(c)) >= AA_TEXT;
	while (!enough(text) && by < 1) {
		by = Math.min(1, by + FILL_STEP);
		text = darken(hex, by);
	}
	return text;
}

/** Sets `--accent` and the colors derived from it in code (`--accent-fill`, its hover, the
 *  `--on-accent` label and `--accent-text`) on `root`; null or an invalid color gives the default. */
export function applyAccent(root: HTMLElement, hex: string | null): void {
	const accent = hex !== null && isHexColor(hex) ? hex : DEFAULT_ACCENT;
	const { fill, hover, text } = accentFill(accent);
	root.style.setProperty('--accent', accent);
	root.style.setProperty('--accent-fill', fill);
	root.style.setProperty('--accent-fill-hover', hover);
	root.style.setProperty('--on-accent', text === 'light' ? 'var(--on-strong)' : 'var(--ink)');
	root.style.setProperty('--accent-text', accentText(accent));
}

/** The soft hues the picker offers after the default, each with the name a screen reader says
 *  (a hex code means nothing read aloud). */
const SWATCH_HUES: readonly { hue: number; name: MessageKey }[] = [
	{ hue: 14, name: 'color_name_coral' },
	{ hue: 38, name: 'color_name_gold' },
	{ hue: 140, name: 'color_name_green' },
	{ hue: 185, name: 'color_name_teal' },
	{ hue: 215, name: 'color_name_blue' },
	{ hue: 265, name: 'color_name_violet' },
	{ hue: 305, name: 'color_name_orchid' }
];

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

/** The accent choices the color picker offers, with their color names: the default accent first,
 *  then soft hues of similar weight. Built here because a literal color may not live in a
 *  component. */
export function swatches(): { hex: string; name: MessageKey }[] {
	return [
		{ hex: DEFAULT_ACCENT, name: 'color_name_rose' },
		...SWATCH_HUES.map(({ hue, name }) => ({ hex: hslToHex(hue, 45, 62), name }))
	];
}
