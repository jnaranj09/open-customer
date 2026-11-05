import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { describe, expect, it } from 'vitest';
import {
	DEFAULT_ACCENT,
	INK_LUMINANCE,
	applyAccent,
	isHexColor,
	luminance,
	onAccent,
	swatches
} from './color';

/** A stand-in for an element: the only thing `applyAccent` touches is the style. */
function fakeRoot(): { root: HTMLElement; vars: Map<string, string> } {
	const vars = new Map<string, string>();
	const root = {
		style: { setProperty: (k: string, v: string) => vars.set(k, v) }
	} as unknown as HTMLElement;
	return { root, vars };
}

describe('onAccent', () => {
	it('picks dark text on a light accent', () => {
		expect(onAccent('#f6e7a1')).toBe('dark');
	});

	it('picks light text on a dark accent', () => {
		expect(onAccent('#a14d64')).toBe('light');
	});

	it('picks light text on the default accent', () => {
		expect(onAccent(DEFAULT_ACCENT)).toBe('light');
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
	it('sets --accent and --on-accent from a valid color', () => {
		const { root, vars } = fakeRoot();
		applyAccent(root, '#f6e7a1');
		expect(Object.fromEntries(vars)).toEqual({
			'--accent': '#f6e7a1',
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
		expect(swatches().every(isHexColor)).toBe(true);
	});

	it('are all different', () => {
		expect(new Set(swatches()).size).toBe(swatches().length);
	});

	it('start from the default accent so her first choice is on the row', () => {
		expect(swatches()[0]).toBe(DEFAULT_ACCENT);
	});
});
