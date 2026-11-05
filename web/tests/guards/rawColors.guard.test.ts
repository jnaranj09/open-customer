// raw-colors (design doc item 88; D10): colors, spacing and radii come from the theme in app.css.
// A hex, rgb(), hsl() or oklch() literal anywhere in shipped source fails, except in app.css and
// the one DEFAULT_ACCENT line in lib/color.ts. A Tailwind arbitrary color or pixel value in a
// .svelte file fails too.
import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import { SRC, srcRel, walk } from './sourceScan';

const THEME_FILE = 'app.css';
const ACCENT_FILE = 'lib/color.ts';
/** The one raw color outside the theme: the accent used before setup (design doc item 64). */
const ACCENT_LINE = /^export const DEFAULT_ACCENT = '#[0-9a-fA-F]{6}';$/;

/** The shell's `theme-color` meta tag needs a literal; it is the only color the HTML may hold. */
const SHELL_FILE = 'app.html';
const THEME_COLOR_LINE = /^\s*<meta name="theme-color" content="#[0-9a-fA-F]{6}" \/>$/;

const COLOR_LITERAL = /#[0-9a-fA-F]{3,8}\b|\b(?:rgba?|hsla?|oklch|oklab|lab|lch|hwb)\(/;
const ARBITRARY = /-\[(?:#|\d+(?:\.\d+)?px\])/;

/** `file:line` of each raw value in `text`, for the file at `rel` (relative to src/). */
export function rawValueHits(rel: string, text: string): string[] {
	if (rel === THEME_FILE) return [];
	const hits: string[] = [];
	text.split('\n').forEach((line, i) => {
		const allowed =
			(rel === ACCENT_FILE && ACCENT_LINE.test(line)) ||
			(rel === SHELL_FILE && THEME_COLOR_LINE.test(line));
		const raw = !allowed && COLOR_LITERAL.test(line);
		const arbitrary = rel.endsWith('.svelte') && ARBITRARY.test(line);
		if (raw || arbitrary) hits.push(`${rel}:${String(i + 1)}`);
	});
	return hits;
}

describe('rawValueHits', () => {
	it('flags a hex, rgb, hsl and oklch literal', () => {
		const text = ['a: #fff;', 'b: rgb(1 2 3);', 'c: hsl(1 2% 3%);', 'd: oklch(0.5 0.1 20);'].join(
			'\n'
		);
		expect(rawValueHits('lib/x.ts', text)).toEqual([
			'lib/x.ts:1',
			'lib/x.ts:2',
			'lib/x.ts:3',
			'lib/x.ts:4'
		]);
	});

	it('flags an arbitrary Tailwind color and pixel value in a .svelte file', () => {
		const text = '<div class="bg-[#fff] w-[13px]"></div>\n<p class="mt-[7px]"></p>';
		expect(rawValueHits('lib/components/X.svelte', text)).toEqual([
			'lib/components/X.svelte:1',
			'lib/components/X.svelte:2'
		]);
	});

	it('allows anything in app.css', () => {
		expect(rawValueHits('app.css', '--bg-100: #fbf5f2;')).toEqual([]);
	});

	it('allows only the DEFAULT_ACCENT line in lib/color.ts', () => {
		const text = "export const DEFAULT_ACCENT = '#c76b85';\nconst other = '#123456';";
		expect(rawValueHits('lib/color.ts', text)).toEqual(['lib/color.ts:2']);
	});

	it('allows only the theme-color meta line in app.html', () => {
		const text = '\t\t<meta name="theme-color" content="#fbf5f2" />\n<p style="color: #fff"></p>';
		expect(rawValueHits('app.html', text)).toEqual(['app.html:2']);
	});

	it('does not extend the DEFAULT_ACCENT allowance to another file', () => {
		expect(rawValueHits('lib/other.ts', "export const DEFAULT_ACCENT = '#c76b85';")).toEqual([
			'lib/other.ts:1'
		]);
	});

	it('does not read a hex-like word as a color', () => {
		expect(rawValueHits('lib/x.ts', 'const abcdefg = 1; // issue #1234567890')).toEqual([]);
	});
});

describe('shipped source', () => {
	it('has no raw color or arbitrary value outside the theme', () => {
		const files = walk(
			SRC,
			(n) => /\.(ts|svelte|css|html)$/.test(n) && !/\.(test|spec)\.ts$/.test(n)
		);
		const hits = files.flatMap((f) => rawValueHits(srcRel(f), readFileSync(f, 'utf8')));
		expect(hits).toEqual([]);
	});
});
