// ui-lint (.claude/rules/ui.md "UI pipeline", stage 4): runs the real ESLint config on in-memory
// probe files and asserts that each way around "only primitives style" is an error, and that
// legitimate layout is not. A probe borrows the path of a real file outside (or inside)
// components/ui, because the typed parser only reads files its project knows; the text linted is
// the probe's, never the file's. A pattern change in eslint.config.js that lets a bypass through
// turns this red.
import { ESLint } from 'eslint';
import { describe, expect, it } from 'vitest';
import { WEB } from './sourceScan';

/** A route with no bulk suppressions, a script outside components/ui, and a primitive. */
const ROUTE = 'src/routes/clientas/+page.svelte';
const SCRIPT = 'src/lib/chipsDom.ts';
const PRIMITIVE = 'src/lib/components/ui/Chip.svelte';

const CARD_IMPORT = "\timport Card from '../../lib/components/ui/Card.svelte';\n";
const svelte = (markup: string, script = ''): string =>
	`<script lang="ts">\n${CARD_IMPORT}${script}</script>\n\n${markup}\n`;

type Probe = { name: string; file: string; code: string; rule: string };

const RESTRICTED = 'better-tailwindcss/no-restricted-classes';
const SYNTAX = 'no-restricted-syntax';

const BYPASSES: Probe[] = [
	{
		name: 'a radius class',
		file: ROUTE,
		code: svelte('<p class="rounded-xl">x</p>'),
		rule: RESTRICTED
	},
	{
		name: 'a leading ! radius',
		file: ROUTE,
		code: svelte('<p class="!rounded-xl">x</p>'),
		rule: RESTRICTED
	},
	{
		name: 'a trailing ! radius',
		file: ROUTE,
		code: svelte('<p class="rounded-xl!">x</p>'),
		rule: RESTRICTED
	},
	{
		name: 'a variant radius',
		file: ROUTE,
		code: svelte('<p class="lg:rounded-xl">x</p>'),
		rule: RESTRICTED
	},
	{
		name: 'glass-card',
		file: ROUTE,
		code: svelte('<p class="glass-card">x</p>'),
		rule: RESTRICTED
	},
	{
		name: 'glass-card!',
		file: ROUTE,
		code: svelte('<p class="glass-card!">x</p>'),
		rule: RESTRICTED
	},
	{
		name: 'a glass fill',
		file: ROUTE,
		code: svelte('<p class="bg-glass-tint">x</p>'),
		rule: RESTRICTED
	},
	{
		name: 'a shadow',
		file: ROUTE,
		code: svelte('<p class="shadow-raised">x</p>'),
		rule: RESTRICTED
	},
	{
		name: 'a drop shadow',
		file: ROUTE,
		code: svelte('<p class="drop-shadow-md">x</p>'),
		rule: RESTRICTED
	},
	{
		name: 'an inset shadow',
		file: ROUTE,
		code: svelte('<p class="inset-shadow-sm">x</p>'),
		rule: RESTRICTED
	},
	{
		name: 'a backdrop filter',
		file: ROUTE,
		code: svelte('<p class="backdrop-blur-md">x</p>'),
		rule: RESTRICTED
	},
	{
		name: 'an arbitrary value',
		file: ROUTE,
		code: svelte('<p class="p-[13px]">x</p>'),
		rule: RESTRICTED
	},
	{
		name: 'a class string in a script variable',
		file: ROUTE,
		code: svelte('<p class={look}>x</p>', "\tconst look = 'flex rounded-xl';\n"),
		rule: RESTRICTED
	},
	{
		name: 'a class string in an object value',
		file: ROUTE,
		code: svelte('<p class={LOOK.a}>x</p>', "\tconst LOOK = { a: 'glass-card' };\n"),
		rule: RESTRICTED
	},
	{
		name: 'a class string in a .ts module',
		file: SCRIPT,
		code: "export const chip = 'inline-block rounded-pill';\n",
		rule: RESTRICTED
	},
	{
		name: 'padding on a Card',
		file: ROUTE,
		code: svelte('<Card class="flex p-4">x</Card>'),
		rule: SYNTAX
	},
	{
		name: 'a leading ! padding on a Card',
		file: ROUTE,
		code: svelte('<Card class="!p-4">x</Card>'),
		rule: SYNTAX
	},
	{
		name: 'a trailing ! padding on a Card',
		file: ROUTE,
		code: svelte('<Card class="p-4!">x</Card>'),
		rule: SYNTAX
	},
	{
		name: 'variant padding on a Card',
		file: ROUTE,
		code: svelte('<Card class="lg:px-4">x</Card>'),
		rule: SYNTAX
	},
	{
		name: 'a non-literal Card class',
		file: ROUTE,
		code: svelte('<Card class={look}>x</Card>', "\tconst look = 'flex';\n"),
		rule: SYNTAX
	},
	{
		name: 'a Card class mixed with an expression',
		file: ROUTE,
		code: svelte("<Card class=\"flex {on ? 'gap-2' : ''}\">x</Card>", '\tconst on = true;\n'),
		rule: SYNTAX
	},
	{
		name: 'a <style> block',
		file: ROUTE,
		code: svelte('<p>x</p>\n<style>p { border-radius: 4px; }</style>'),
		rule: SYNTAX
	},
	{
		name: 'an arbitrary value in a primitive',
		file: PRIMITIVE,
		code: '<p class="max-h-[3px]">x</p>\n',
		rule: RESTRICTED
	}
];

const CLEAN: Omit<Probe, 'rule'>[] = [
	{
		name: 'layout and padding on plain elements in a route',
		file: ROUTE,
		code: svelte(
			'<div class="flex flex-col gap-4 px-4 lg:mt-2"><Card pad="md" class="flex gap-2">x</Card></div>'
		)
	},
	{
		name: 'radius and surface in a primitive',
		file: PRIMITIVE,
		code: '<p class="glass-card rounded-xl shadow-raised">x</p>\n'
	}
];

const eslint = new ESLint({ cwd: WEB });
const rulesHit = async (p: Omit<Probe, 'rule'>): Promise<(string | null)[]> => {
	const [result] = await eslint.lintText(p.code, { filePath: p.file });
	return (result?.messages ?? []).map((m) => m.ruleId);
};

describe('ui lint probes', () => {
	it.each(BYPASSES)(
		'rejects $name',
		async (p) => {
			expect(await rulesHit(p)).toContain(p.rule);
		},
		60_000
	);

	it.each(CLEAN)(
		'allows $name',
		async (p) => {
			expect(await rulesHit(p)).toEqual([]);
		},
		60_000
	);
});
