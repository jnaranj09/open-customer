// The hand-off from Cotizar to Nueva cita (quick-quotes design, section 4): the quote's services
// become the Nueva cita draft, which that form reads once when it opens.
import { blankForm, FormSchema, type Line } from '../booking';
import { getDraft, putDraft } from './db';
import { draftKey } from './drafts';

/** Writes `{ ...blankForm(today), lines }` as the `cita:new` draft. A draft already there asks
 *  first (`needs_confirm`) unless `replace` is true. The write is awaited and goes straight to
 *  Dexie, not through the 500 ms autosave: Nueva cita could open before a delayed write lands. */
export async function convertQuote(
	lines: Line[],
	today: string,
	replace: boolean
): Promise<'needs_confirm' | 'ok' | 'failed'> {
	const key = draftKey('cita', 'new');
	try {
		// A draft that fails its schema reads as none.
		if (!replace && (await getDraft(key, FormSchema)) !== null) {
			return 'needs_confirm';
		}
		await putDraft(key, FormSchema.parse({ ...blankForm(today), lines }));
		return 'ok';
	} catch {
		console.warn('code=quote_convert_failed');
		return 'failed';
	}
}
