import js from '@eslint/js';
import vitest from '@vitest/eslint-plugin';
import playwright from 'eslint-plugin-playwright';
import svelte from 'eslint-plugin-svelte';
import globals from 'globals';
import { defineConfig } from 'eslint/config';
import ts from 'typescript-eslint';
import viteConfig from './vite.config.ts';

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
