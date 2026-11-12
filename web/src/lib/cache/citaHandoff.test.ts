import 'fake-indexeddb/auto';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { blankForm, FormSchema, type Line } from '../booking';
import { clearCache, getDraft, putDraft } from './db';
import { draftKey } from './drafts';
import { handOff } from './citaHandoff';

const KEY = draftKey('cita', 'new');
const TODAY = '2026-11-08';
const lines: Line[] = [
	{ service_id: 'svc-a', count: 1 },
	{ service_id: 'svc-b', count: 2 }
];
/** A half-written booking, as Nueva cita saves it. */
const ana = { ...blankForm('2026-11-09'), clientId: 'client-ana', notes: 'trae diseño' };

beforeEach(async () => {
	await clearCache();
});

describe('handOff', () => {
	it('no draft writes lines and today', async () => {
		expect(await handOff({ ...blankForm(TODAY), lines }, false)).toBe('ok');

		expect(await getDraft(KEY, FormSchema)).toEqual({ ...blankForm(TODAY), lines });
	});

	it('existing draft asks first and stays', async () => {
		await putDraft(KEY, ana);

		expect(await handOff({ ...blankForm(TODAY), lines }, false)).toBe('needs_confirm');

		expect(await getDraft(KEY, FormSchema)).toEqual(ana);
	});

	it('replace overwrites', async () => {
		await putDraft(KEY, ana);

		expect(await handOff({ ...blankForm(TODAY), lines }, true)).toBe('ok');

		expect(await getDraft(KEY, FormSchema)).toEqual({ ...blankForm(TODAY), lines });
	});

	it('a draft that fails its schema counts as none', async () => {
		await putDraft(KEY, { lines: 'not an array' });

		expect(await handOff({ ...blankForm(TODAY), lines }, false)).toBe('ok');

		expect(await getDraft(KEY, FormSchema)).toEqual({ ...blankForm(TODAY), lines });
	});

	it('a failed write returns failed and keeps the draft', async () => {
		await putDraft(KEY, ana);
		const warn = vi.spyOn(console, 'warn').mockImplementation(() => undefined);
		vi.spyOn(IDBObjectStore.prototype, 'put').mockImplementation(() => {
			throw new DOMException('full', 'QuotaExceededError');
		});

		// Dexie surfaces the throw as a rejection, which is what handOff catches.
		await expect(putDraft(KEY, ana)).rejects.toThrow();

		const result = await handOff({ ...blankForm(TODAY), lines }, true);

		vi.restoreAllMocks();
		expect(result).toBe('failed');
		expect(warn).toHaveBeenCalledWith('code=cita_handoff_failed');
		expect(await getDraft(KEY, FormSchema)).toEqual(ana);
	});

	it('writes the whole form, not only the lines', async () => {
		const form = { ...blankForm('2026-12-03'), clientId: 'client-ana', lines, time: '14:30' };

		expect(await handOff(form, false)).toBe('ok');

		expect(await getDraft(KEY, FormSchema)).toEqual(form);
	});

	it('a form that fails its schema returns failed and writes nothing', async () => {
		const warn = vi.spyOn(console, 'warn').mockImplementation(() => undefined);
		const bad = { ...blankForm(TODAY), lines: [{ service_id: 'svc-a', count: 0 }] };

		expect(await handOff(bad, true)).toBe('failed');

		expect(warn).toHaveBeenCalledWith('code=cita_handoff_failed');
		expect(await getDraft(KEY, FormSchema)).toBeNull();
	});
});
