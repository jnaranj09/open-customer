// test-hygiene (principle P17), the parts lint cannot see: mocks only at process edges, fake timers
// turned off in the same file, temp folders removed, no test file over MAX_LINES.
import { readFileSync } from 'node:fs';
import ts from 'typescript';
import { describe, expect, it } from 'vitest';
import { WEB, parseSource, relTo, testFiles } from './sourceScan';

const MAX_LINES = 1000;

/** Lines in a file: a trailing newline ends the last line, it does not start another. */
export function countLines(text: string): number {
	return text.split('\n').length - (text.endsWith('\n') ? 1 : 0);
}

/** Module ids a test may mock: the process edges (network, filesystem, clock, OS, other
 *  processes). IndexedDB is faked with fake-indexeddb, which is imported, not mocked. */
export function isEdge(id: string): boolean {
	const EDGE_PACKAGES: readonly string[] = ['fs', 'pg'];
	return id.startsWith('node:') || EDGE_PACKAGES.includes(id);
}

/** Internal mocks that existed when the guard landed: `<test file>: <module id>`. Only shrinks. */
const INTERNAL_MOCKS: readonly string[] = [];

/** `vi.mock('<id>')` / `vi.doMock('<id>')` first arguments that are string literals. */
export function mockTargets(sf: ts.SourceFile): string[] {
	const out: string[] = [];
	const visit = (n: ts.Node): void => {
		if (
			ts.isCallExpression(n) &&
			ts.isPropertyAccessExpression(n.expression) &&
			ts.isIdentifier(n.expression.expression) &&
			n.expression.expression.text === 'vi' &&
			(n.expression.name.text === 'mock' || n.expression.name.text === 'doMock')
		) {
			const a = n.arguments[0];
			if (a !== undefined && ts.isStringLiteralLike(a)) out.push(a.text);
		}
		ts.forEachChild(n, visit);
	};
	visit(sf);
	return out;
}

/** True when the file calls a function (bare or as a member) whose name is in `names`. */
export function callsNamed(sf: ts.SourceFile, names: readonly string[]): boolean {
	let found = false;
	const visit = (n: ts.Node): void => {
		if (found) return;
		if (ts.isCallExpression(n)) {
			const e = n.expression;
			const name = ts.isIdentifier(e)
				? e.text
				: ts.isPropertyAccessExpression(e)
					? e.name.text
					: '';
			if (names.includes(name)) found = true;
		}
		ts.forEachChild(n, visit);
	};
	visit(sf);
	return found;
}

const FAKE_ON = ['useFakeTimers'];
const FAKE_OFF = ['useRealTimers'];
const TMP_MAKE = ['mkdtemp', 'mkdtempSync'];
const TMP_REMOVE = ['rm', 'rmSync', 'rmdir', 'rmdirSync'];

const probe = (text: string): ts.SourceFile => parseSource('probe.test.ts', text);

describe('probes', () => {
	it('reads vi.mock and vi.doMock targets, not comments', () => {
		expect(mockTargets(probe("vi.mock('../x')\nvi.doMock('fs')\n// vi.mock('../y')"))).toEqual([
			'../x',
			'fs'
		]);
	});
	it('countLines does not count the newline that ends the last line', () => {
		expect(countLines('x\n'.repeat(MAX_LINES))).toBe(MAX_LINES);
	});
	it('callsNamed sees bare and member calls, not comments', () => {
		expect(callsNamed(probe('vi.useFakeTimers()'), FAKE_ON)).toBe(true);
		expect(callsNamed(probe('// vi.useRealTimers()'), FAKE_OFF)).toBe(false);
	});
});

describe('test hygiene', () => {
	const parsed = testFiles().map((f) => {
		const text = readFileSync(f, 'utf8');
		return { file: relTo(WEB, f), text, sf: parseSource(f, text) };
	});

	it('walks the test files (a walk that finds none is broken)', () => {
		expect(parsed.length).toBeGreaterThan(0);
	});

	it('mocks only process edges, beyond the listed internal mocks (the list only shrinks)', () => {
		const internal = parsed.flatMap(({ file, sf }) =>
			mockTargets(sf)
				.filter((id) => !isEdge(id))
				.map((id) => `${file}: ${id}`)
		);
		expect(internal.sort()).toEqual([...INTERNAL_MOCKS].sort());
	});

	it('a file that turns on fake timers turns them off', () => {
		const bad = parsed.filter(({ sf }) => callsNamed(sf, FAKE_ON) && !callsNamed(sf, FAKE_OFF));
		expect(bad.map((p) => p.file)).toEqual([]);
	});

	it('a file that makes a temp folder removes one', () => {
		const bad = parsed.filter(({ sf }) => callsNamed(sf, TMP_MAKE) && !callsNamed(sf, TMP_REMOVE));
		expect(bad.map((p) => p.file)).toEqual([]);
	});

	it(`no test file is over ${String(MAX_LINES)} lines`, () => {
		const big = parsed.filter(({ text }) => countLines(text) > MAX_LINES);
		expect(big.map((p) => p.file)).toEqual([]);
	});
});
