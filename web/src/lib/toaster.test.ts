import { readFileSync, readdirSync } from 'node:fs';
import { join, relative } from 'node:path';
import { describe, expect, it } from 'vitest';
import { createToaster } from './toaster.svelte';

describe('toaster', () => {
	it('shows a new message in place of the one on screen', () => {
		const toaster = createToaster();
		toaster.say('client_saved');
		toaster.say('photo_added');
		expect(toaster.current?.message).toBe('Foto agregada');
	});

	it('fills the message parameters', () => {
		const toaster = createToaster();
		toaster.say('due_skipped', undefined, { name: 'Ana' });
		expect(toaster.current?.message).toContain('Ana');
	});

	it('dismisses the toast it names', () => {
		const toaster = createToaster();
		toaster.dismiss(toaster.say('client_saved'));
		expect(toaster.current).toBeNull();
	});

	it('leaves a newer toast when an older one is dismissed', () => {
		const toaster = createToaster();
		const old = toaster.say('client_saved');
		toaster.say('photo_added');
		toaster.dismiss(old);
		expect(toaster.current?.message).toBe('Foto agregada');
	});
});

const SRC = join(import.meta.dirname, '..');

/** The `.svelte` files under src that draw a `<Toast`. */
function toastHosts(): string[] {
	return readdirSync(SRC, { recursive: true, encoding: 'utf8' })
		.filter((p) => p.endsWith('.svelte'))
		.filter((p) => readFileSync(join(SRC, p), 'utf8').includes('<Toast'))
		.map((p) => relative(SRC, join(SRC, p)));
}

describe('the toast host', () => {
	it('is the root layout alone, so two toasts never show at once', () => {
		expect(toastHosts()).toEqual(['routes/+layout.svelte']);
	});
});
