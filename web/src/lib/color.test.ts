import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { describe, expect, it } from 'vitest';
import {
	AA_TEXT,
	DEFAULT_ACCENT,
	INK_LUMINANCE,
	accentFill,
	applyAccent,
	contrast,
	isHexColor,
	luminance,
	swatches
} from './color';
import { t } from './i18n';

/** A stand-in for an element: the only thing `applyAccent` touches is the style. */
function fakeRoot(): { root: HTMLElement; vars: Map<string, string> } {
	const vars = new Map<string, string>();
	const root = {
		style: { setProperty: (k: string, v: string) => vars.set(k, v) }
	} as unknown as HTMLElement;
	return { root, vars };
}

/** The contrast of the label on the fill `accentFill` gives for `hex`. */
function labelContrast(hex: string): number {
	const { fill, text } = accentFill(hex);
	return contrast(text === 'light' ? 1 : INK_LUMINANCE, luminance(fill));
}

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
		expect(accentFill('#a0a0a0')).toEqual({ fill: '#707070', text: 'light' });
	});

	it('keeps a dark accent with white text', () => {
		expect(accentFill('#000000')).toEqual({ fill: '#000000', text: 'light' });
	});

	it('reaches AA for every accent on a grid of the color cube', () => {
		const steps = ['00', '33', '66', '99', 'cc', 'ff'];
		const all = steps.flatMap((r) => steps.flatMap((g) => steps.map((b) => `#${r}${g}${b}`)));
		const low = [...all, ...swatches().map((s) => s.hex)].filter(
			(hex) => labelContrast(hex) < AA_TEXT
		);
		expect(low).toEqual([]);
	});

	it('matches the --accent-fill default in app.css', () => {
		const css = readFileSync(join(import.meta.dirname, '../app.css'), 'utf8');
		expect(/--accent-fill:\s*(#[0-9a-f]{6})/i.exec(css)?.[1]).toBe(accentFill(DEFAULT_ACCENT).fill);
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
	it('sets --accent, --accent-fill and --on-accent from a valid color', () => {
		const { root, vars } = fakeRoot();
		applyAccent(root, '#f6e7a1');
		expect(Object.fromEntries(vars)).toEqual({
			'--accent': '#f6e7a1',
			'--accent-fill': accentFill('#f6e7a1').fill,
			'--on-accent': 'var(--ink)'
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
