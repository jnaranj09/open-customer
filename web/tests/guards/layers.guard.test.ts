// layers (principle P9): lib never imports routes, shipped code never imports test helpers, and the
// service worker imports only the SvelteKit worker modules and lib modules.
import { join } from 'node:path';
import { describe, expect, it } from 'vitest';
import { SRC, importSpecifiers, layerViolations, parseSource, sourceFiles } from './sourceScan';

const at = (rel: string, text: string): string[] =>
	layerViolations(join(SRC, rel), importSpecifiers(parseSource(join(SRC, rel), text)));

describe('layer rules', () => {
	it('flags lib importing a route, by relative path', () => {
		expect(at('lib/a.ts', "import x from '../routes/+page.svelte';\n")).toEqual([
			'lib/a.ts → ../routes/+page.svelte'
		]);
	});

	it('flags a component importing a route through a type import, a re-export and import()', () => {
		expect(
			at(
				'lib/components/B.svelte',
				"<script lang=\"ts\">\nimport type { T } from '../../routes/t';\nexport * from '../../routes/x';\nvoid import('../../routes/y');\n</script>\n"
			)
		).toEqual([
			'lib/components/B.svelte → ../../routes/t',
			'lib/components/B.svelte → ../../routes/x',
			'lib/components/B.svelte → ../../routes/y'
		]);
	});

	it('flags shipped code importing a test helper', () => {
		expect(at('routes/+page.ts', "import { seed } from '../../tests/e2e/seed';\n")).toEqual([
			'routes/+page.ts → ../../tests/e2e/seed'
		]);
	});

	it('flags the service worker importing anything but the worker modules and lib', () => {
		expect(
			at(
				'service-worker.ts',
				"import { assets } from '$app/manifest';\nimport { t } from '$lib/i18n';\nimport { goto } from '$app/navigation';\nimport { x } from './routes/x';\n"
			)
		).toEqual(['service-worker.ts → $app/navigation', 'service-worker.ts → ./routes/x']);
	});

	it('allows routes importing lib and SvelteKit modules', () => {
		expect(
			at(
				'routes/+page.svelte',
				"<script>\nimport { t } from '$lib/i18n';\nimport { browser } from '$app/environment';\n</script>\n"
			)
		).toEqual([]);
	});

	it('does not read an import inside a comment or a string', () => {
		expect(
			at('lib/c.ts', "// import x from '../routes/x';\nconst s = \"import '../routes/y'\";\n")
		).toEqual([]);
	});
});

describe('layer guard', () => {
	it('walks real files (a walk that finds none is broken)', () => {
		expect(sourceFiles().length).toBeGreaterThan(0);
	});

	it('the real tree breaks no layer rule', () => {
		const hits = sourceFiles({ declarations: true }).flatMap((f) =>
			layerViolations(f, importSpecifiers(parseSource(f)))
		);
		expect(hits).toEqual([]);
	});
});
