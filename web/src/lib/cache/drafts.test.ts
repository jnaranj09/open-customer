import 'fake-indexeddb/auto';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { z } from 'zod';
import { clearCache, putDraft } from './db';
import { autosave, draftKey, draftStep } from './drafts';

const Form = z.object({ name: z.string() });

beforeEach(async () => {
	await clearCache();
	// Only the timer: fake-indexeddb schedules its work with setImmediate.
	vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] });
});

afterEach(() => {
	vi.useRealTimers();
});

describe('draftKey', () => {
	it('joins the form and the record id', () => {
		expect(draftKey('cita', 'new')).toBe('cita:new');
	});

	it('keeps the record id for an edit', () => {
		expect(draftKey('clienta', '3f2b8c1e')).toBe('clienta:3f2b8c1e');
	});
});

describe('draftStep', () => {
	it('waits until loaded', () => {
		expect(draftStep(false, { a: 1 }, { a: 1 })).toBe('wait');
		expect(draftStep(false, { a: 2 }, { a: 1 })).toBe('wait');
	});

	it('clears when equal to initial', () => {
		expect(draftStep(true, { a: [1] }, { a: [1] })).toBe('clear');
	});

	it('saves when different', () => {
		expect(draftStep(true, { a: 2 }, { a: 1 })).toBe('save');
	});
});

describe('autosave', () => {
	it('writes 500 ms after the last input', async () => {
		const draft = autosave('cita:new', Form);
		draft.save({ name: 'Ana' });
		await vi.advanceTimersByTimeAsync(300);
		draft.save({ name: 'Ana Rojas' });
		await vi.advanceTimersByTimeAsync(499);
		expect(await draft.load()).toBeNull();
		await vi.advanceTimersByTimeAsync(1);
		expect(await draft.load()).toEqual({ name: 'Ana Rojas' });
	});

	it('restores a draft saved earlier', async () => {
		await putDraft('cita:new', { name: 'Marta Solano' });
		expect(await autosave('cita:new', Form).load()).toEqual({ name: 'Marta Solano' });
	});

	it('clear removes the draft', async () => {
		const draft = autosave('cita:new', Form);
		draft.save({ name: 'Ana' });
		await vi.advanceTimersByTimeAsync(500);
		await draft.clear();
		expect(await draft.load()).toBeNull();
	});

	it('clear cancels a save that has not fired yet', async () => {
		const draft = autosave('cita:new', Form);
		draft.save({ name: 'Ana' });
		await draft.clear();
		await vi.advanceTimersByTimeAsync(1000);
		expect(await draft.load()).toBeNull();
	});

	it('a stored draft that fails its schema loads null', async () => {
		vi.spyOn(console, 'warn').mockImplementation(() => undefined);
		await putDraft('cita:new', { name: 42 });
		expect(await autosave('cita:new', Form).load()).toBeNull();
	});
});
