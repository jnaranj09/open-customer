// forbid-only (principle P3): a focused test fails the run. Vitest needs `allowOnly: false` (and
// `restoreMocks: true`, so a spy never leaks into the next test); Playwright needs
// `forbidOnly: true` once its config exists (design doc item 79, P6).
import { existsSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import ts from 'typescript';
import { describe, expect, it } from 'vitest';
import config from '../../vite.config';
import { WEB } from './sourceScan';

/** The literal value of every `<name>: <true|false>` property in a config file. */
export function booleanProps(text: string, name: string): boolean[] {
	const sf = ts.createSourceFile('config.ts', text, ts.ScriptTarget.Latest, true);
	const out: boolean[] = [];
	const visit = (n: ts.Node): void => {
		if (
			ts.isPropertyAssignment(n) &&
			ts.isIdentifier(n.name) &&
			n.name.text === name &&
			(n.initializer.kind === ts.SyntaxKind.TrueKeyword ||
				n.initializer.kind === ts.SyntaxKind.FalseKeyword)
		) {
			out.push(n.initializer.kind === ts.SyntaxKind.TrueKeyword);
		}
		ts.forEachChild(n, visit);
	};
	visit(sf);
	return out;
}

const PLAYWRIGHT = join(WEB, 'playwright.config.ts');
const playwrightText = existsSync(PLAYWRIGHT) ? readFileSync(PLAYWRIGHT, 'utf8') : null;

describe('booleanProps', () => {
	it('reads literal true and false, not comments or other values', () => {
		expect(
			booleanProps(
				'export default { forbidOnly: true, x: { forbidOnly: false }, y: { forbidOnly: !!1 } } // forbidOnly: true',
				'forbidOnly'
			)
		).toEqual([true, false]);
	});
});

describe('vitest config', () => {
	it('has no inline projects (each would need its own settings)', () => {
		expect(config.test?.projects).toBeUndefined();
	});

	it('sets allowOnly: false and restoreMocks: true', () => {
		expect({ allowOnly: config.test?.allowOnly, restoreMocks: config.test?.restoreMocks }).toEqual({
			allowOnly: false,
			restoreMocks: true
		});
	});
});

describe('playwright config', () => {
	it('sets forbidOnly: true, and nothing else, once it exists', () => {
		expect(playwrightText === null ? [true] : booleanProps(playwrightText, 'forbidOnly')).toEqual([
			true
		]);
	});
});
