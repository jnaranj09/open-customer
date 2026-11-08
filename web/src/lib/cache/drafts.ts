// Drafts of the new-appointment, new-client and quote forms (design doc item 67). The row lives in
// `db.ts`; this module only decides when to write it.
import type { z } from 'zod';
import { deleteDraft, getDraft, putDraft } from './db';

/** The Dexie key of a form's draft: `<form>:<record id>`, with `new` as the id of a form that has
 *  no record yet. */
export function draftKey(form: 'cita' | 'clienta' | 'cotizacion', id: string): string {
	return `${form}:${id}`;
}

/** What a draft form does with its current value: `wait` until the saved draft is loaded, `clear`
 *  when the value is back to the initial one (JSON equality), else `save`. */
export function draftStep<T>(loaded: boolean, now: T, initial: T): 'wait' | 'save' | 'clear' {
	if (!loaded) {
		return 'wait';
	}
	return JSON.stringify(now) === JSON.stringify(initial) ? 'clear' : 'save';
}

/** A draft saved `delayMs` after the last input (500 ms), loaded and cleared by key. `load` parses
 *  the stored value with `schema`, so a draft from an older form shape never reaches the form. */
export function autosave<T>(
	key: string,
	schema: z.ZodType<T>,
	delayMs = 500
): { save(v: T): void; load(): Promise<T | null>; clear(): Promise<void> } {
	let timer: ReturnType<typeof setTimeout> | undefined;
	return {
		save(v: T): void {
			clearTimeout(timer);
			timer = setTimeout(() => {
				putDraft(key, v).catch(() => {
					console.warn('code=draft_save_failed');
				});
			}, delayMs);
		},
		load: () => getDraft(key, schema),
		async clear(): Promise<void> {
			clearTimeout(timer);
			await deleteDraft(key);
		}
	};
}
