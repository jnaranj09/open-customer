import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { describe, expect, it } from 'vitest';
import {
	AA_TEXT,
	BG_100_RGB,
	DEFAULT_ACCENT,
	GLASS_TINT_ALPHA,
	GLOW_BLUSH_SHARE,
	GLOW_LILAC_RGB,
	GLOW_PEACH_RGB,
	INK_LUMINANCE,
	accent100,
	accent300,
	accentFill,
	accentText,
	applyAccent,
	contrast,
	darkestPage,
	glassTint,
	glowBlush,
	isHexColor,
	luminance,
	swatches
} from './color';
import { t } from './i18n';

const STEPS = ['00', '33', '66', '99', 'cc', 'ff'];
/** 216 colors spread over the sRGB cube. */
const CUBE = STEPS.flatMap((r) => STEPS.flatMap((g) => STEPS.map((b) => `#${r}${g}${b}`)));

/** A stand-in for an element: the only thing `applyAccent` touches is the style. */
function fakeRoot(): { root: HTMLElement; vars: Map<string, string> } {
	const vars = new Map<string, string>();
	const root = {
		style: { setProperty: (k: string, v: string) => vars.set(k, v) }
	} as unknown as HTMLElement;
	return { root, vars };
}

/** The contrast of the label on the fill (or its hover) `accentFill` gives for `hex`. */
function labelContrast(hex: string, on: 'fill' | 'hover' = 'fill'): number {
	const r = accentFill(hex);
	return contrast(r.text === 'light' ? 1 : INK_LUMINANCE, luminance(r[on]));
}

const BG_100 = `#${BG_100_RGB.map((v) => v.toString(16).padStart(2, '0')).join('')}`;

const hexOf = (rgb: readonly number[]): string =>
	`#${rgb.map((v) => v.toString(16).padStart(2, '0')).join('')}`;

/** The lowest contrast of `accentText(hex)` on the surfaces accent text sits on: the page
 *  background and each glow center, `--accent-100`, `--accent-300` (a ghost or accent icon
 *  button's hover), and the glass tint (a status chip) over the page and over each glow. */
function textContrast(hex: string): number {
	const l = luminance(accentText(hex));
	const tintOver = (page: string): string => overHex(accent300(hex), 0.35, page);
	const pages = [BG_100, hexOf(GLOW_PEACH_RGB), hexOf(GLOW_LILAC_RGB), glowBlush(hex)];
	return Math.min(
		...[...pages, ...pages.map(tintOver), accent100(hex), accent300(hex)].map((s) =>
			contrast(luminance(s), l)
		)
	);
}

const CSS = (): string => readFileSync(join(import.meta.dirname, '../app.css'), 'utf8');
const cssColor = (name: string): string | undefined =>
	new RegExp(`--${name}:\\s*(#[0-9a-f]{6});`, 'i').exec(CSS())?.[1];

describe('accentFill', () => {
	it('gives the default accent a fill white text reaches AA on (white on it is 3.57:1)', () => {
		expect(accentFill(DEFAULT_ACCENT).text).toBe('light');
		expect(labelContrast(DEFAULT_ACCENT)).toBeGreaterThanOrEqual(AA_TEXT);
	});

	it('picks ink text on a light yellow accent', () => {
		expect(accentFill('#f6e7a1').text).toBe('dark');
		expect(labelContrast('#f6e7a1')).toBeGreaterThanOrEqual(AA_TEXT);
	});

	it('darkens a mid-tone accent past its 700 shade until white text reaches AA', () => {
		// #a0a0a0 at the 700 shade (#808080) gives white 3.95:1 and ink 2.89:1.
		expect(accentFill('#a0a0a0')).toMatchObject({ fill: '#707070', text: 'light' });
	});

	it('keeps a dark accent with white text', () => {
		expect(accentFill('#000000')).toMatchObject({ fill: '#000000', text: 'light' });
	});

	it('reaches AA for every accent on a grid of the color cube, on the fill and its hover', () => {
		const low = [...CUBE, ...swatches().map((s) => s.hex)].filter(
			(hex) => labelContrast(hex) < AA_TEXT || labelContrast(hex, 'hover') < AA_TEXT
		);
		expect(low).toEqual([]);
	});

	it('gives a hover fill that differs from the fill', () => {
		expect(CUBE.filter((hex) => accentFill(hex).hover === accentFill(hex).fill)).toEqual([]);
	});

	it('matches the --accent-fill and --accent-fill-hover defaults in app.css', () => {
		const { fill, hover } = accentFill(DEFAULT_ACCENT);
		expect([cssColor('accent-fill'), cssColor('accent-fill-hover')]).toEqual([fill, hover]);
	});
});

describe('accentText', () => {
	it.each(['#c76b85', '#f6e7a1', '#ffff00'])(
		'reaches AA on the page, the accent tints, the ghost hover and the glass tint for %s',
		(hex) => {
			expect(textContrast(hex)).toBeGreaterThanOrEqual(AA_TEXT);
		}
	);

	it('reaches AA for every accent on a grid of the color cube', () => {
		expect(CUBE.filter((hex) => textContrast(hex) < AA_TEXT)).toEqual([]);
	});

	it('matches the --accent-text default in app.css', () => {
		expect(cssColor('accent-text')).toBe(accentText(DEFAULT_ACCENT));
	});
});

describe('accent100', () => {
	it('mixes 15% of the accent into white, as color-mix in oklab does', () => {
		expect([accent100('#ffffff'), accent100('#000000')]).toEqual(['#ffffff', '#cecece']);
	});
});

describe('accent300', () => {
	it('mixes 45% of the accent into white, as color-mix in oklab does', () => {
		expect([accent300('#ffffff'), accent300('#808080')]).toEqual(['#ffffff', '#c4c4c4']);
	});

	it('mixes less of a near-black accent in, so black text keeps AA on the hover fill', () => {
		expect(contrast(0, luminance(accent300('#000000')))).toBeGreaterThanOrEqual(AA_TEXT);
	});
});

describe('glassTint', () => {
	it('paints a white accent-300 at 35% over the lilac glow, the darkest page then', () => {
		expect(glassTint('#ffffff')).toBe(overHex('#ffffff', 0.35, hexOf(GLOW_LILAC_RGB)));
	});

	it('paints a grey accent-300 at 35% over the darkest page, channel by channel', () => {
		expect(glassTint('#808080')).toBe(overHex('#c4c4c4', 0.35, darkestPage('#808080')));
	});
});

/** `top` at `alpha` over `page`, worked out apart from the code. */
function overHex(top: string, alpha: number, page: string): string {
	const ch = (h: string, i: number): number => Number.parseInt(h.slice(1 + 2 * i, 3 + 2 * i), 16);
	return hexOf([0, 1, 2].map((i) => Math.round(ch(top, i) * alpha + ch(page, i) * (1 - alpha))));
}

describe('glowBlush', () => {
	it('mixes 25% of the accent into white, as color-mix in oklab does', () => {
		expect([glowBlush('#ffffff'), glowBlush('#808080')]).toEqual(['#ffffff', '#dedede']);
	});
});

describe('darkestPage', () => {
	it('is the lilac glow for the default accent, darker than the page', () => {
		expect(darkestPage(DEFAULT_ACCENT)).toBe(hexOf(GLOW_LILAC_RGB));
	});

	it('is the blush glow for a dark accent', () => {
		expect(darkestPage('#000000')).toBe(glowBlush('#000000'));
	});
});

describe('glow channels', () => {
	it('match --glow-peach and --glow-lilac in app.css', () => {
		expect([hexOf(GLOW_PEACH_RGB), hexOf(GLOW_LILAC_RGB)]).toEqual([
			cssColor('glow-peach'),
			cssColor('glow-lilac')
		]);
	});

	it('match the share of the accent in --glow-blush in app.css', () => {
		const share = /--glow-blush:\s*color-mix\(in oklab, var\(--accent\) (\d+)%, white\);/.exec(
			CSS()
		)?.[1];
		expect(Number(share) / 100).toBe(GLOW_BLUSH_SHARE);
	});
});

describe('BG_100_RGB', () => {
	it('matches the --bg-100 token in app.css', () => {
		expect(BG_100).toBe(cssColor('bg-100'));
	});
});

describe('GLASS_TINT_ALPHA', () => {
	it('matches the share of --accent-300 in the --glass-tint token in app.css', () => {
		const share =
			/--glass-tint:\s*color-mix\(in oklab, var\(--accent-300\) (\d+)%, transparent\);/.exec(
				CSS()
			)?.[1];
		expect(Number(share) / 100).toBe(GLASS_TINT_ALPHA);
	});
});

describe('isHexColor', () => {
	it('rejects short and named colors', () => {
		expect([isHexColor('#fff'), isHexColor('red'), isHexColor('')]).toEqual([false, false, false]);
	});

	it('accepts #rrggbb in either case', () => {
		expect([isHexColor('#C76B85'), isHexColor('#c76b85')]).toEqual([true, true]);
	});

	it('rejects an alpha channel and trailing text', () => {
		expect([isHexColor('#c76b85ff'), isHexColor('#c76b85 ')]).toEqual([false, false]);
	});
});

describe('luminance', () => {
	it('is 1 for white and 0 for black', () => {
		expect([luminance('#ffffff'), luminance('#000000')]).toEqual([1, 0]);
	});
});

describe('INK_LUMINANCE', () => {
	it('matches the --ink token in app.css', () => {
		const css = readFileSync(join(import.meta.dirname, '../app.css'), 'utf8');
		const ink = /--ink:\s*(#[0-9a-f]{6})/i.exec(css)?.[1];
		expect(ink).toBeDefined();
		expect(INK_LUMINANCE).toBeCloseTo(luminance(ink ?? ''), 6);
	});
});

describe('DEFAULT_ACCENT', () => {
	it('matches the --accent token in app.css', () => {
		const css = readFileSync(join(import.meta.dirname, '../app.css'), 'utf8');
		expect(/--accent:\s*(#[0-9a-f]{6})/i.exec(css)?.[1]).toBe(DEFAULT_ACCENT);
	});
});

describe('applyAccent', () => {
	it('sets --accent and every color derived from it in code from a valid color', () => {
		const { root, vars } = fakeRoot();
		applyAccent(root, '#f6e7a1');
		expect(Object.fromEntries(vars)).toEqual({
			'--accent': '#f6e7a1',
			'--accent-fill': accentFill('#f6e7a1').fill,
			'--accent-fill-hover': accentFill('#f6e7a1').hover,
			'--on-accent': 'var(--ink)',
			'--accent-text': accentText('#f6e7a1'),
			'--accent-300': accent300('#f6e7a1')
		});
	});

	it('uses white text on a dark accent', () => {
		const { root, vars } = fakeRoot();
		applyAccent(root, '#a14d64');
		expect(vars.get('--on-accent')).toBe('var(--on-strong)');
	});

	it('falls back to the default accent on null', () => {
		const { root, vars } = fakeRoot();
		applyAccent(root, null);
		expect(vars.get('--accent')).toBe(DEFAULT_ACCENT);
	});

	it('falls back to the default accent on an invalid color', () => {
		const { root, vars } = fakeRoot();
		applyAccent(root, 'red');
		expect(vars.get('--accent')).toBe(DEFAULT_ACCENT);
	});
});

describe('swatches', () => {
	it('are all #rrggbb', () => {
		expect(swatches().every((s) => isHexColor(s.hex))).toBe(true);
	});

	it('are all different', () => {
		expect(new Set(swatches().map((s) => s.hex)).size).toBe(swatches().length);
	});

	it('start from the default accent so her first choice is on the row', () => {
		expect(swatches()[0]?.hex).toBe(DEFAULT_ACCENT);
	});

	it('each have their own Spanish color name, not a hex code', () => {
		const names = swatches().map((s) => t(s.name));
		expect(new Set(names).size).toBe(names.length);
		expect(names.filter((n) => n.includes('#'))).toEqual([]);
	});
});
