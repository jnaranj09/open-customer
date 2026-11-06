import js from '@eslint/js';
import vitest from '@vitest/eslint-plugin';
import betterTailwind from 'eslint-plugin-better-tailwindcss';
import { getDefaultSelectors } from 'eslint-plugin-better-tailwindcss/defaults';
import playwright from 'eslint-plugin-playwright';
import svelte from 'eslint-plugin-svelte';
import globals from 'globals';
import { defineConfig } from 'eslint/config';
import ts from 'typescript-eslint';
import viteConfig from './vite.config.ts';

// Only primitives style (.claude/rules/ui.md "UI pipeline", stage 4). Arbitrary values (`p-[13px]`,
// `[mask:x]`) are banned everywhere: a value the theme lacks becomes a token or an @utility in
// app.css. Outside components/ui, surface and radius classes are banned too: a glass surface is a
// `Card` (or another primitive), and a radius belongs to the primitive that draws the shape.
// Layout (flex, grid, gap, width, margin, position) and padding stay allowed in routes and feature
// components; padding on a `Card` is not (the `pad` prop sets it; `no-restricted-syntax` below).
// Patterns match the whole class, variants included (`lg:rounded-xl`), so each starts at `^` or
// `:`; the important modifier is allowed on either side (`!rounded-xl`, `rounded-xl!`). Outside
// components/ui the rule also reads every string in a script variable (`const look = '…'`), not
// only the plugin's default names. The probes in tests/guards/uiLint.guard.test.ts prove each ban.
const ARBITRARY = {
	pattern: '\\[.*\\]',
	message: 'No arbitrary value "$0": add a token or an @utility in app.css (ui.md stage 4).'
};
const PRIMITIVE_ONLY = [
	ARBITRARY,
	{
		pattern:
			'(^|:)!?(glass-card|glass-panel|bg-glass(-strong|-tint)?|([\\w]+-)?shadow(-[\\w-]+)?|backdrop-[\\w-]+)!?$',
		message:
			'Surface class "$0" outside components/ui: use Card or another primitive (ui.md stage 4).'
	},
	{
		pattern: '(^|:)!?rounded(-[\\w-]+)?!?$',
		message:
			'Radius class "$0" outside components/ui: the primitive owns the shape (ui.md stage 4).'
	}
];
/** The plugin's selectors plus every string in any script variable or object value. */
const EVERY_CLASS_STRING = [
	...getDefaultSelectors(),
	{ kind: 'variable', name: '.*', match: [{ type: 'strings' }, { type: 'objectValues' }] }
];
const CARD_CLASS =
	"SvelteElement[kind='component'][name.name='Card'] > SvelteStartTag > SvelteAttribute[key.name='class']";

export default defineConfig(
	{ ignores: ['build/', '.svelte-kit/', 'node_modules/', 'src/lib/api/schema.d.ts'] },
	js.configs.recommended,
	ts.configs.strictTypeChecked,
	svelte.configs.recommended,
	{
		languageOptions: {
			globals: { ...globals.browser, ...globals.node },
			parserOptions: {
				projectService: true,
				extraFileExtensions: ['.svelte'],
				tsconfigRootDir: import.meta.dirname
			}
		}
	},
	{
		// The worker has its own tsconfig (the webworker lib), outside the one the project service finds.
		files: ['src/service-worker.ts', 'src/lib/sw-routes.ts'],
		languageOptions: {
			parserOptions: { projectService: false, project: './tsconfig.service-worker.json' }
		}
	},
	{
		files: ['**/*.svelte', '**/*.svelte.ts', '**/*.svelte.js'],
		languageOptions: {
			parserOptions: { parser: ts.parser, svelteConfig: viteConfig }
		}
	},
	{
		rules: {
			'@typescript-eslint/no-explicit-any': 'error',
			'@typescript-eslint/no-floating-promises': 'error',
			'@typescript-eslint/consistent-type-imports': 'error',
			'@typescript-eslint/switch-exhaustiveness-check': 'error'
		}
	},
	{
		files: ['src/**/*.svelte', 'src/**/*.ts'],
		plugins: { 'better-tailwindcss': betterTailwind },
		settings: { 'better-tailwindcss': { entryPoint: 'src/app.css' } },
		rules: {
			...betterTailwind.configs.correctness.rules,
			'better-tailwindcss/no-restricted-classes': ['error', { restrict: [ARBITRARY] }]
		}
	},
	{
		files: ['src/**/*.svelte', 'src/**/*.ts'],
		ignores: ['src/lib/components/ui/**'],
		rules: {
			'better-tailwindcss/no-restricted-classes': [
				'error',
				{ restrict: PRIMITIVE_ONLY, selectors: EVERY_CLASS_STRING }
			],
			'no-restricted-syntax': [
				'error',
				{
					selector: `${CARD_CLASS} SvelteLiteral[value=/(^|\\s)([\\w@-]+:)*!?-?p[xytrblse]?-/]`,
					message: 'Padding on a Card comes from its `pad` prop, not a class (ui.md stage 4).'
				},
				{
					selector: `${CARD_CLASS} > SvelteMustacheTag`,
					message:
						'A Card class is a plain string (layout only), so lint can read it; state goes in a prop (ui.md stage 4).'
				},
				{
					selector: 'SvelteStyleElement',
					message:
						'No <style> outside components/ui: style lives in tokens and primitives (ui.md stage 4).'
				}
			]
		}
	},
	{
		files: ['src/**/*.test.ts', 'tests/guards/**/*.test.ts'],
		plugins: { vitest },
		rules: {
			...vitest.configs.recommended.rules,
			'vitest/valid-title': 'error',
			'vitest/expect-expect': 'error',
			'vitest/no-conditional-in-test': 'error',
			'vitest/no-conditional-expect': 'error',
			'vitest/no-focused-tests': 'error',
			'vitest/no-disabled-tests': 'error',
			'vitest/no-restricted-matchers': [
				'error',
				{
					toMatchSnapshot: 'No snapshots (rules/testing.md).',
					toMatchInlineSnapshot: 'No snapshots (rules/testing.md).',
					toThrowErrorMatchingSnapshot: 'No snapshots (rules/testing.md).',
					toThrowErrorMatchingInlineSnapshot: 'No snapshots (rules/testing.md).'
				}
			]
		}
	},
	{
		files: ['tests/e2e/**/*.ts'],
		...playwright.configs['flat/recommended'],
		rules: {
			...playwright.configs['flat/recommended'].rules,
			'playwright/no-skipped-test': 'error',
			'playwright/no-wait-for-timeout': 'error',
			'playwright/no-focused-test': 'error',
			'playwright/no-raw-locators': 'error',
			'playwright/prefer-native-locators': 'error'
		}
	}
);
