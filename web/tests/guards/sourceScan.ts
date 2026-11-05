// The one source scanner behind the web guards (TASKS.md T6). It reads code the way the compiler
// does, through the TypeScript AST (and the Svelte parser for .svelte scripts), so a comment or a
// string that only looks like an import or a catch never counts.
import { readFileSync, readdirSync } from 'node:fs';
import { dirname, join, relative, resolve, sep } from 'node:path';
import { parse, type AST } from 'svelte/compiler';
import ts from 'typescript';

/** The web package root. */
export const WEB = resolve(import.meta.dirname, '../..');
/** Root of shipped source. */
export const SRC = join(WEB, 'src');
/** Test helpers and guards; shipped code never imports from here. */
export const TESTS = join(WEB, 'tests');

/** Path relative to `base`, always with `/`. */
export const relTo = (base: string, file: string): string =>
	relative(base, file).split(sep).join('/');
export const srcRel = (file: string): string => relTo(SRC, file);

const isTest = (name: string): boolean => /\.(test|spec)\.ts$/.test(name);

/** Every file under `dir` whose name passes `keep`, sorted. */
export function walk(dir: string, keep: (name: string) => boolean): string[] {
	const out: string[] = [];
	const visit = (d: string): void => {
		for (const e of readdirSync(d, { withFileTypes: true })) {
			const p = join(d, e.name);
			if (e.isDirectory()) {
				if (e.name !== 'node_modules' && !e.name.startsWith('.')) visit(p);
			} else if (keep(e.name)) out.push(p);
		}
	};
	visit(dir);
	return out.sort();
}

/** Every shipped source file: .ts and .svelte, no tests, no .d.ts unless asked. */
export function sourceFiles(options: { declarations?: boolean } = {}): string[] {
	return walk(
		SRC,
		(n) =>
			(n.endsWith('.svelte') || n.endsWith('.ts')) &&
			!isTest(n) &&
			(options.declarations === true || !n.endsWith('.d.ts'))
	);
}

/** Every test file under src/ and tests/. */
export function testFiles(): string[] {
	return [...walk(SRC, isTest), ...walk(TESTS, isTest)];
}

/** The TypeScript inside a file: the whole file, or a .svelte file's <script> blocks. */
export function scriptText(file: string, text: string): string {
	if (!file.endsWith('.svelte')) return text;
	const ast = parse(text, { modern: true });
	// The parser leaves a missing <script> undefined, although the type says null.
	const scripts: (AST.Script | null | undefined)[] = [ast.module, ast.instance];
	return scripts
		.flatMap((s) => {
			if (s === null || s === undefined) return [];
			// The typed AST leaves out the script body's offsets; the parser sets them.
			const start: unknown = Reflect.get(s.content, 'start');
			const end: unknown = Reflect.get(s.content, 'end');
			if (typeof start !== 'number' || typeof end !== 'number') {
				throw new Error('svelte parse: <script> body has no offsets');
			}
			return [text.slice(start, end)];
		})
		.join('\n');
}

export function parseSource(file: string, text = readFileSync(file, 'utf8')): ts.SourceFile {
	return ts.createSourceFile(
		file,
		scriptText(file, text),
		ts.ScriptTarget.Latest,
		true,
		ts.ScriptKind.TS
	);
}

/** Module specifiers of every import, re-export, `import type`, `import('x')` type and dynamic
 *  `import('x')` in the file. */
export function importSpecifiers(sf: ts.SourceFile): string[] {
	const out: string[] = [];
	const visit = (node: ts.Node): void => {
		if (
			(ts.isImportDeclaration(node) || ts.isExportDeclaration(node)) &&
			node.moduleSpecifier !== undefined &&
			ts.isStringLiteral(node.moduleSpecifier)
		) {
			out.push(node.moduleSpecifier.text);
		} else if (
			ts.isImportTypeNode(node) &&
			ts.isLiteralTypeNode(node.argument) &&
			ts.isStringLiteral(node.argument.literal)
		) {
			out.push(node.argument.literal.text);
		} else if (
			ts.isCallExpression(node) &&
			node.expression.kind === ts.SyntaxKind.ImportKeyword &&
			node.arguments[0] !== undefined &&
			ts.isStringLiteral(node.arguments[0])
		) {
			out.push(node.arguments[0].text);
		}
		ts.forEachChild(node, visit);
	};
	visit(sf);
	return out;
}

/** Where a specifier points: an absolute path for `$lib` and relative imports, else null (a
 *  package or a SvelteKit module such as `$app/environment`). */
function target(file: string, spec: string): string | null {
	if (spec === '$lib' || spec.startsWith('$lib/')) return join(SRC, 'lib', spec.slice(4));
	if (spec.startsWith('.')) return resolve(dirname(file), spec);
	return null;
}

// SvelteKit 3 replaced `$service-worker` with these three.
const WORKER_MODULES: ReadonlySet<string> = new Set([
	'$app/env',
	'$app/manifest',
	'$app/service-worker'
]);

const isUnder = (path: string, dir: string): boolean => path === dir || path.startsWith(dir + sep);

/** `<file> → <specifier>` for each import that breaks the web layer rules (design doc "Code layout";
 *  TASKS.md T6):
 *  - lib (components included) never imports routes;
 *  - shipped code never imports test helpers;
 *  - the service worker imports only the SvelteKit worker modules (`WORKER_MODULES`) and lib modules. */
export function layerViolations(file: string, specifiers: readonly string[]): string[] {
	const from = srcRel(file);
	const inLib = from.startsWith('lib/');
	const isWorker = from === 'service-worker.ts' || from.startsWith('service-worker/');
	const hits: string[] = [];
	for (const spec of specifiers) {
		const to = target(file, spec);
		const bad =
			(to !== null && isUnder(to, TESTS)) ||
			(inLib && to !== null && isUnder(to, join(SRC, 'routes'))) ||
			(isWorker && !WORKER_MODULES.has(spec) && (to === null || !isUnder(to, join(SRC, 'lib'))));
		if (bad) hits.push(`${from} → ${spec}`);
	}
	return hits;
}

/** Catch blocks with no statement: `empty` (nothing at all), `commented` (only a comment) and,
 *  of those, `unexplained` (the comment is under 3 words). */
export function silentCatches(sf: ts.SourceFile): {
	empty: number;
	commented: number;
	unexplained: number;
} {
	const text = sf.getFullText();
	const r = { empty: 0, commented: 0, unexplained: 0 };
	const visit = (node: ts.Node): void => {
		if (ts.isCatchClause(node) && node.block.statements.length === 0) {
			const inner = text.slice(node.block.getStart(sf) + 1, node.block.getEnd() - 1);
			const words = inner
				.replace(/\/\*|\*\/|\/\//g, ' ')
				.split(/\s+/)
				.filter((w) => /\w/.test(w));
			if (words.length === 0) r.empty++;
			else {
				r.commented++;
				if (words.length < 3) r.unexplained++;
			}
		}
		ts.forEachChild(node, visit);
	};
	visit(sf);
	return r;
}

/** The text of every comment in TypeScript source, found with the TS scanner (so a string that
 *  contains `//` or `eslint-disable` is not a comment). */
export function comments(code: string): string[] {
	const scanner = ts.createScanner(
		ts.ScriptTarget.Latest,
		false,
		ts.LanguageVariant.Standard,
		code
	);
	const out: string[] = [];
	for (let k = scanner.scan(); k !== ts.SyntaxKind.EndOfFileToken; k = scanner.scan()) {
		if (k === ts.SyntaxKind.SingleLineCommentTrivia || k === ts.SyntaxKind.MultiLineCommentTrivia) {
			out.push(scanner.getTokenText());
		}
	}
	return out;
}

/** HTML comments in a .svelte file's markup (where `<!-- eslint-disable -->` also works). */
export function markupComments(file: string, text: string): string[] {
	if (!file.endsWith('.svelte')) return [];
	const out: string[] = [];
	const visit = (node: unknown): void => {
		if (typeof node !== 'object' || node === null) return;
		if (Reflect.get(node, 'type') === 'Comment') {
			const data: unknown = Reflect.get(node, 'data');
			if (typeof data === 'string') out.push(data);
		}
		for (const v of Object.values(node)) {
			if (Array.isArray(v)) v.forEach(visit);
			else if (typeof v === 'object') visit(v);
		}
	};
	visit(parse(text, { modern: true }).fragment);
	return out;
}
