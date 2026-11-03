// catch-blocks (principle P12): a contained error is logged with a `code=`, or the catch says why
// dropping it is safe. A catch with no statement is allowed only where ALLOWED names the file, at
// the exact count, with the reason. The list only shrinks.
import { join } from 'node:path';
import { describe, expect, it } from 'vitest';
import { SRC, parseSource, silentCatches, sourceFiles, srcRel } from './sourceScan';

/** File under src/ → exact count + why dropping is safe. Only shrinks. */
const ALLOWED: Record<string, { count: number; why: string }> = {};

const scan = (text: string): ReturnType<typeof silentCatches> =>
	silentCatches(parseSource(join(SRC, 'x.ts'), text));

describe('silentCatches', () => {
	it('counts a truly empty catch as empty', () => {
		expect(scan('try { f() } catch {}\n')).toEqual({ empty: 1, commented: 0, unexplained: 0 });
	});

	it('counts a catch holding only a reason as commented', () => {
		expect(scan('try { f() } catch {\n  /* fail open: the list still renders */\n}\n')).toEqual({
			empty: 0,
			commented: 1,
			unexplained: 0
		});
	});

	it('flags a comment too short to say why (fewer than 3 words)', () => {
		expect(scan('try { f() } catch {\n  /* ignore */\n}\n')).toEqual({
			empty: 0,
			commented: 1,
			unexplained: 1
		});
	});

	it('does not count a catch that logs or continues', () => {
		expect(
			scan("try { f() } catch { log('code=x') }\nfor (;;) { try { f() } catch { continue } }\n")
		).toEqual({ empty: 0, commented: 0, unexplained: 0 });
	});

	it('reads the script of a .svelte file', () => {
		const sf = parseSource(
			join(SRC, 'X.svelte'),
			'<script>\ntry { f() } catch {}\n</script>\n<p>x</p>\n'
		);
		expect(silentCatches(sf).empty).toBe(1);
	});
});

describe('silent-catch guard', () => {
	const files = sourceFiles();
	const found = new Map<string, ReturnType<typeof silentCatches>>();
	for (const f of files) {
		const r = silentCatches(parseSource(f));
		if (r.empty + r.commented > 0) found.set(srcRel(f), r);
	}

	it('walks real files (a walk that finds none is broken)', () => {
		expect(files.length).toBeGreaterThan(0);
	});

	it('no catch is truly empty, and every comment-only catch says why', () => {
		const bad = [...found].filter(([, r]) => r.empty + r.unexplained > 0).map(([f]) => f);
		expect(bad).toEqual([]);
	});

	it('comment-only catches exist only where the list allows, at the listed count', () => {
		const actual = Object.fromEntries([...found].map(([f, r]) => [f, r.commented]));
		const expected = Object.fromEntries(Object.entries(ALLOWED).map(([f, a]) => [f, a.count]));
		expect(actual).toEqual(expected);
	});
});
