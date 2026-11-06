// suppression-budget (principle P19). Two totals are pinned and matched EXACTLY, so they can only
// fall, and a fix lowers the pin in the same commit:
// - the ESLint bulk suppressions file (eslint-suppressions.json), which `--suppress-all` could grow;
// - inline suppression comments (eslint-disable*, @ts-ignore, @ts-expect-error, @ts-nocheck and
//   Svelte's svelte-ignore) in source, tests and config, counted with the TS scanner (and the Svelte parser
//   for markup) so a string that mentions one does not count. A silenced a11y warning or a styling
//   class let into a route raises a total.
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { describe, expect, it } from 'vitest';
import { SRC, TESTS, WEB, comments, markupComments, scriptText, walk } from './sourceScan';

/** Measured when each strict rule landed (TASKS.md T4; better-tailwindcss/no-restricted-classes:
 *  69 radius and surface classes in routes and feature components when "only primitives style"
 *  landed (ui.md stage 4), 35 after TextButton, Callout, SelectField and fieldStyle took the
 *  patterns with 3 or more copies, 29 after the Chip trailing control and IconButton square).
 *  Only ever lowered. */
const PINNED_BULK_TOTAL = 29;
const PINNED_INLINE_TOTAL = 0;

/** Sum of every `count` in an ESLint bulk-suppressions file; throws on any other shape. */
export function suppressionTotal(json: unknown): number {
	if (typeof json !== 'object' || json === null) throw new Error('suppressions: not an object');
	let total = 0;
	const files: unknown[] = Object.values(json);
	for (const rules of files) {
		if (typeof rules !== 'object' || rules === null)
			throw new Error('suppressions: bad file entry');
		const entries: unknown[] = Object.values(rules);
		for (const entry of entries) {
			const count: unknown =
				typeof entry === 'object' && entry !== null ? Reflect.get(entry, 'count') : undefined;
			if (typeof count !== 'number' || !Number.isInteger(count) || count < 1)
				throw new Error('suppressions: bad count');
			total += count;
		}
	}
	return total;
}

// A directive starts the comment; a comment that only mentions one (like this file's header) is not
// one.
const SUPPRESSION =
	/^(?:\/\/|\/\*)?[\s*]*(?:eslint-disable|@ts-ignore|@ts-expect-error|@ts-nocheck|svelte-ignore)/;

/** Inline suppression comments in one file. */
export function inlineSuppressions(file: string, text: string): number {
	return [...comments(scriptText(file, text)), ...markupComments(file, text)].filter((c) =>
		SUPPRESSION.test(c)
	).length;
}

describe('suppressionTotal', () => {
	it('sums every count', () => {
		expect(
			suppressionTotal({
				'a.test.ts': { 'rule/x': { count: 2 }, 'rule/y': { count: 1 } },
				'b.test.ts': { 'rule/x': { count: 4 } }
			})
		).toBe(7);
	});
	it('throws on a shape it does not know', () => {
		expect(() => suppressionTotal({ 'a.test.ts': { 'rule/x': { n: 2 } } })).toThrow(/bad count/);
	});
});

describe('inlineSuppressions', () => {
	it('counts comments, not strings, in TS and in .svelte script and markup', () => {
		expect(
			inlineSuppressions(
				'a.ts',
				"// eslint-disable-next-line no-console\nconst s = '// @ts-ignore';\n/* @ts-expect-error */\n// see eslint-disable docs\n"
			)
		).toBe(2);
		expect(
			inlineSuppressions(
				'A.svelte',
				'<script>\n// @ts-ignore\nlet x = 1;\n</script>\n<!-- eslint-disable-next-line -->\n<p>{x}</p>\n'
			)
		).toBe(2);
	});
	it('counts a svelte-ignore in markup and script, not a comment that only mentions one', () => {
		expect(
			inlineSuppressions(
				'A.svelte',
				'<script>\n// svelte-ignore state_referenced_locally\nlet x = 1;\n</script>\n<!-- svelte-ignore a11y_autofocus -->\n<!-- see the svelte-ignore docs -->\n<p>{x}</p>\n'
			)
		).toBe(2);
	});
});

describe('suppression budget', () => {
	it('the bulk file total equals PINNED_BULK_TOTAL', () => {
		const json: unknown = JSON.parse(readFileSync(join(WEB, 'eslint-suppressions.json'), 'utf8'));
		expect(suppressionTotal(json)).toBe(PINNED_BULK_TOTAL);
	});

	it('the inline total equals PINNED_INLINE_TOTAL', () => {
		const code = (n: string): boolean => /\.(ts|js|svelte)$/.test(n);
		const files = [
			...walk(SRC, code),
			...walk(TESTS, code),
			...['vite.config.ts', 'eslint.config.js'].map((f) => join(WEB, f))
		];
		expect(files.length).toBeGreaterThan(2);
		const total = files.reduce((n, f) => n + inlineSuppressions(f, readFileSync(f, 'utf8')), 0);
		expect(total).toBe(PINNED_INLINE_TOTAL);
	});
});
