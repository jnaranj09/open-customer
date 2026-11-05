// design-tokens (.claude/rules/ui.md "UI pipeline", stage 2): the `.pen` file owns every token; the
// CSS theme follows it. Fails on a token missing on either side, or a value that differs. A token
// the browser computes at run time (a `var()` chain, `color-mix()`, a value set from settings)
// cannot be compared as a literal: it goes on ALLOWED with a reason. ALLOWED only shrinks, and an
// entry that no longer matches anything fails, so a fixed drift leaves the list in the same change.
// CSS is read with postcss, never a regex, so a comment that mentions `--x: 1px` is not a token.
// Only top-level declarations are compared: a themed override (`@media { :root { … } }`,
// `@supports`, `@container`) is skipped, so a second theme is not checked yet.
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { AtRule, Rule, parse } from 'postcss';
import { describe, expect, it } from 'vitest';
import { MAP_FILE, PEN_FILE, parseDesignMap, parsePen, readJson } from './designSource';
import type { PenValue } from './designSource';
import { SRC } from './sourceScan';

/** The CSS theme. */
const THEME_FILE = join(SRC, 'app.css');
/** Where tokens are declared, at the top level of the file: rules with these selectors, at-rules
 *  with these names (`@theme`). */
const TOKEN_SELECTORS: readonly string[] = [':root'];
const TOKEN_AT_RULES: readonly string[] = ['theme'];

type Allowed = Record<string, { kind: 'pen-only' | 'css-only' | 'derived'; why: string }>;

const ACCENT =
	'follows the runtime accent (applyAccent in lib/color.ts sets --accent from settings)';
const SPACING = "Tailwind's spacing scale (--spacing, 4px) gives every step; no token of its own";

/** Measured when the guard landed. Key: the `.pen` name (pen-only, derived) or the CSS property
 *  (css-only). Only shrinks. */
const ALLOWED: Allowed = {
	'space-1': { kind: 'pen-only', why: SPACING },
	'space-2': { kind: 'pen-only', why: SPACING },
	'space-3': { kind: 'pen-only', why: SPACING },
	'space-4': { kind: 'pen-only', why: SPACING },
	'space-6': { kind: 'pen-only', why: SPACING },
	'space-8': { kind: 'pen-only', why: SPACING },
	'space-12': { kind: 'pen-only', why: SPACING },
	'font-display': { kind: 'pen-only', why: 'the mockup display face; the app ships Nunito only' },
	'font-sans': { kind: 'derived', why: 'CSS names the variable font and adds a fallback' },
	'rose-100': { kind: 'derived', why: ACCENT },
	'rose-300': { kind: 'derived', why: ACCENT },
	'rose-700': { kind: 'derived', why: ACCENT },
	'glass-tint': { kind: 'derived', why: ACCENT },
	'glow-blush': { kind: 'derived', why: ACCENT },
	'--shadow-glass': { kind: 'css-only', why: 'the .pen keeps shadows as node effects' },
	'--shadow-raised': { kind: 'css-only', why: 'the .pen keeps shadows as node effects' },
	'--shadow-sheet': { kind: 'css-only', why: 'the .pen keeps shadows as node effects' }
};

/** Custom properties declared in the top-level token scopes, name → value. An alias
 *  (`--color-x: var(--x)` where `--x` is itself a token, as the `@theme inline` bridge does) is
 *  not a token of its own. */
export function cssTokens(css: string): Map<string, string> {
	const all = new Map<string, string>();
	parse(css).walkDecls((decl) => {
		if (!decl.prop.startsWith('--')) return;
		const p = decl.parent;
		// Top level: the block's parents up to the Root are only `@layer`; inside `@media`,
		// `@supports` or `@container` it is a themed override.
		let up = p?.parent;
		while (up instanceof AtRule && up.name === 'layer') up = up.parent;
		const topLevel = up?.type === 'root';
		const inScope =
			topLevel &&
			((p instanceof Rule && TOKEN_SELECTORS.includes(p.selector)) ||
				(p instanceof AtRule && TOKEN_AT_RULES.includes(p.name)));
		if (!inScope) return;
		const value = decl.value.trim();
		const seen = all.get(decl.prop);
		if (seen !== undefined && seen !== value) throw new Error(`css: ${decl.prop} declared twice`);
		all.set(decl.prop, value);
	});
	const out = new Map<string, string>();
	for (const [prop, value] of all) {
		const alias = /^var\((--[\w-]+)\)$/.exec(value)?.[1];
		if (alias === undefined || !all.has(alias)) out.set(prop, value);
	}
	return out;
}

/** Whether a CSS literal equals a `.pen` value: colors ignore case, a number matches `<n>px` or
 *  `<n>`, a string matches with quotes removed. */
export function sameValue(pen: PenValue, css: string): boolean {
	if (typeof pen === 'number') return css === `${String(pen)}px` || css === String(pen);
	if (typeof pen === 'boolean') return css === String(pen);
	return pen.toLowerCase() === css.replace(/^(['"])(.*)\1$/, '$2').toLowerCase();
}

export function tokenDrift(
	pen: Map<string, { value: PenValue }>,
	css: Map<string, string>,
	rename: Record<string, string>,
	allowed: Allowed
): { missingInCss: string[]; missingInPen: string[]; mismatched: string[]; stale: string[] } {
	const missingInCss: string[] = [];
	const missingInPen: string[] = [];
	const mismatched: string[] = [];
	const used = new Set<string>();
	const claimed = new Set<string>();
	for (const [name, { value }] of pen) {
		const prop = rename[name] ?? `--${name}`;
		claimed.add(prop);
		const cssValue = css.get(prop);
		const allow = allowed[name];
		if (cssValue === undefined) {
			if (allow?.kind === 'pen-only') used.add(name);
			else missingInCss.push(`${name} → ${prop}`);
		} else if (!sameValue(value, cssValue)) {
			if (allow?.kind === 'derived') used.add(name);
			else mismatched.push(`${name}: pen ${String(value)} ≠ css ${cssValue}`);
		}
	}
	for (const prop of css.keys()) {
		if (claimed.has(prop)) continue;
		if (allowed[prop]?.kind === 'css-only') used.add(prop);
		else missingInPen.push(prop);
	}
	const stale = Object.keys(allowed).filter((k) => !used.has(k));
	return { missingInCss, missingInPen, mismatched, stale };
}

const clean = { missingInCss: [], missingInPen: [], mismatched: [], stale: [] };

const probePen = new Map<string, { value: PenValue }>([
	['ink', { value: '#1F2933' }],
	['radius-md', { value: 16 }],
	['surface-fill', { value: '#ffffff8c' }]
]);
const probeCss = new Map([
	['--ink', '#1f2933'],
	['--radius-md', '16px'],
	['--surface', '#ffffff8c']
]);
const probeRename = { 'surface-fill': '--surface' };

describe('cssTokens', () => {
	it('reads top-level :root and @theme, skipping aliases, other selectors, overrides and comments', () => {
		const css = [
			':root { --ink: #1f2933; --surface: #ffffff8c; }',
			'@theme inline { --color-ink: var(--ink); --radius-md: 16px; }',
			'.dark { --ink: #000; }',
			'@media (prefers-color-scheme: dark) { :root { --ink: #000; } }',
			'/* --fake: 1px; */'
		].join('\n');
		expect([...cssTokens(css)]).toEqual([
			['--ink', '#1f2933'],
			['--surface', '#ffffff8c'],
			['--radius-md', '16px']
		]);
	});

	it('throws on a top-level token declared twice with two values', () => {
		expect(() => cssTokens(':root { --ink: #000; }\n@theme { --ink: #111; }')).toThrow(/twice/);
	});
});

describe('tokenDrift', () => {
	it('is clean when names map and values agree', () => {
		expect(tokenDrift(probePen, probeCss, probeRename, {})).toEqual(clean);
	});

	it('flags a token missing on each side and a value that differs', () => {
		const drifted = new Map([...probeCss, ['--extra', '1px'], ['--radius-md', '12px']]);
		drifted.delete('--ink');
		expect(tokenDrift(probePen, drifted, probeRename, {})).toEqual({
			missingInCss: ['ink → --ink'],
			missingInPen: ['--extra'],
			mismatched: ['radius-md: pen 16 ≠ css 12px'],
			stale: []
		});
	});

	it('flags an allow-list entry that is no longer needed', () => {
		const allowed: Allowed = { ink: { kind: 'derived', why: 'probe' } };
		expect(tokenDrift(probePen, probeCss, probeRename, allowed).stale).toEqual(['ink']);
	});
});

describe('design-tokens guard', () => {
	// Read inside the tests, not at module load: a parse failure is one red test with its message.
	const load = (): {
		pen: Map<string, { value: PenValue }>;
		rename: Record<string, string>;
		css: Map<string, string>;
	} => ({
		pen: parsePen(readJson(PEN_FILE)).variables,
		rename: parseDesignMap(readJson(MAP_FILE)).tokens,
		css: cssTokens(readFileSync(THEME_FILE, 'utf8'))
	});

	it('reads real tokens on both sides (a walk that finds none is broken)', () => {
		const { pen, css } = load();
		expect(pen.size).toBeGreaterThan(0);
		expect(css.size).toBeGreaterThan(0);
	});

	it('the .pen file and the CSS theme agree (fix the other side, not ALLOWED)', () => {
		const { pen, rename, css } = load();
		expect(tokenDrift(pen, css, rename, ALLOWED)).toEqual(clean);
	});
});
