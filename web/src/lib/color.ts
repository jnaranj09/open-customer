import type { MessageKey } from './i18n';

/** The accent before setup and when settings have none (the mockup's rose-500). Mirrors `--accent`
 *  in app.css. */
export const DEFAULT_ACCENT = '#c76b85';

/** Relative luminance (WCAG 2.x) of `--ink` (app.css). A test keeps the two in sync, so
 *  no second copy of the color lives in code. */
export const INK_LUMINANCE = 0.041973526925469073;

/** The red, green and blue of `--bg-100` (app.css), the page background; a test keeps them in
 *  sync. Channels, not a luminance: the glass tint is composed over it. */
export const BG_100_RGB: readonly [number, number, number] = [251, 245, 242];

/** The red, green and blue of `--glow-peach` and `--glow-lilac` (app.css), two of the soft glows
 *  the body background draws over `--bg-100`; a test keeps them in sync. The lilac one is darker
 *  than the page, so a chip over it is darker too. */
export const GLOW_PEACH_RGB: readonly [number, number, number] = [251, 225, 207];
export const GLOW_LILAC_RGB: readonly [number, number, number] = [231, 220, 243];

/** The share of the accent in `--glow-blush` (app.css: `color-mix(in oklab, var(--accent) 25%,
 *  white)`), the third glow; a test keeps it in sync. */
export const GLOW_BLUSH_SHARE = 0.25;

/** The alpha of `--glass-tint` (app.css: `color-mix(in oklab, var(--accent-300) 35%,
 *  transparent)`); a test keeps it in sync. */
export const GLASS_TINT_ALPHA = 0.35;

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

/** `share` (0 to 1) of `hex` mixed into white in OKLab, as `color-mix(in oklab, …, white)` does. */
function mixWhite(hex: string, share: number): string {
	const [L, A, B] = toOklab(hex);
	// White is L 1, a 0, b 0 in OKLab.
	return fromOklab([share * L + (1 - share), share * A, share * B]);
}

/** `--accent-100` (app.css: `color-mix(in oklab, var(--accent) 15%, white)`) for `hex`. */
export function accent100(hex: string): string {
	return mixWhite(hex, 0.15);
}

/** The share of the accent in `--accent-300` (app.css: `color-mix(in oklab, var(--accent) 45%,
 *  white)`), and the step it falls by for a near-black accent. */
const SHARE_300 = 0.45;
const SHARE_STEP = 0.05;

/** `--accent-300` for `hex`: the accent mixed 45% into white, as app.css does. It is a hover fill
 *  under accent text, so for a near-black accent, where even black text would miss AA on that mix,
 *  the share falls until black reaches AA; `applyAccent` sets the result. */
export function accent300(hex: string): string {
	let share = SHARE_300;
	let mix = mixWhite(hex, share);
	while (contrast(0, luminance(mix)) < AA_TEXT && share > 0) {
		share = Math.max(0, share - SHARE_STEP);
		mix = mixWhite(hex, share);
	}
	return mix;
}

const rgbHex = (rgb: readonly number[]): string =>
	`#${rgb.map((v) => Math.round(v).toString(16).padStart(2, '0')).join('')}`;

/** What a browser paints for `top` at `alpha` over the opaque `bottom`: source-over, channel by
 *  channel, in sRGB. */
function over(top: string, alpha: number, bottom: string): string {
	const t = Number.parseInt(top.slice(1), 16);
	const b = Number.parseInt(bottom.slice(1), 16);
	return rgbHex([16, 8, 0].map((s) => ((t >> s) & 0xff) * alpha + ((b >> s) & 0xff) * (1 - alpha)));
}

/** `--glow-blush` (app.css: `color-mix(in oklab, var(--accent) 25%, white)`) for `hex`. */
export function glowBlush(hex: string): string {
	return mixWhite(hex, GLOW_BLUSH_SHARE);
}

/** The darkest the page behind a surface gets for `hex`: `--bg-100` or the center of one of the
 *  body's radial glows (peach, lilac, and the blush that follows the accent). Where glows overlap
 *  they blend, which never goes darker than the darkest of them. */
export function darkestPage(hex: string): string {
	const pages = [
		rgbHex(BG_100_RGB),
		rgbHex(GLOW_PEACH_RGB),
		rgbHex(GLOW_LILAC_RGB),
		glowBlush(hex)
	];
	return pages.reduce((a, b) => (luminance(b) < luminance(a) ? b : a));
}

/** The `bg-glass-tint` surface (a status chip, the archived badge, the offline banner) as it is
 *  painted for `hex`: color-mix with `transparent` keeps `--accent-300` at 35% alpha, drawn here
 *  over the darkest page behind it (`darkestPage`, a glow included). A glass card between them
 *  only lightens it, so this is the darkest case for dark text. */
export function glassTint(hex: string): string {
	return over(accent300(hex), GLASS_TINT_ALPHA, darkestPage(hex));
}

/** The accent as text on a light surface (a ghost button, a text button, a selected chip, a status
 *  chip, the times in the grid): the accent's 700 shade, darkened until it reaches AA on each
 *  surface it sits on: the page at its darkest (`darkestPage`), the pale accent tint (`--accent-100`),
 *  the hover fill of a ghost button and an accent icon button (`--accent-300`) and the glass tint
 *  (`glassTint`). */
export function accentText(hex: string): string {
	const surfaces = [darkestPage(hex), accent100(hex), accent300(hex), glassTint(hex)].map(
		luminance
	);
	let by = FILL_DARKEN;
	let text = darken(hex, by);
	const enough = (c: string): boolean =>
		surfaces.every((s) => contrast(s, luminance(c)) >= AA_TEXT);
	while (!enough(text) && by < 1) {
		by = Math.min(1, by + FILL_STEP);
		text = darken(hex, by);
	}
	return text;
}

/** Sets `--accent` and the colors derived from it in code (`--accent-fill`, its hover, the
 *  `--on-accent` label, `--accent-text` and `--accent-300`) on `root`; null or an invalid color
 *  gives the default. */
export function applyAccent(root: HTMLElement, hex: string | null): void {
	const accent = hex !== null && isHexColor(hex) ? hex : DEFAULT_ACCENT;
	const { fill, hover, text } = accentFill(accent);
	root.style.setProperty('--accent', accent);
	root.style.setProperty('--accent-fill', fill);
	root.style.setProperty('--accent-fill-hover', hover);
	root.style.setProperty('--on-accent', text === 'light' ? 'var(--on-strong)' : 'var(--ink)');
	root.style.setProperty('--accent-text', accentText(accent));
	root.style.setProperty('--accent-300', accent300(accent));
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
