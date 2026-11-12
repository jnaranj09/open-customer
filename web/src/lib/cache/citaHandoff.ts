// The hand-off into Nueva cita: a form filled somewhere else becomes the Nueva cita draft, which
// that form reads once when it opens. Two callers: Cotizar (quick-quotes design, section 4) and
// "Agendar próxima cita" (next-appointment design, section 5).
import { FormSchema, type Form } from '../booking';
import { getDraft, putDraft } from './db';
import { draftKey } from './drafts';

/** Writes `form` as the `cita:new` draft. A draft already there asks first (`needs_confirm`)
 *  unless `replace` is true. The write is awaited and goes straight to Dexie, not through the
 *  500 ms autosave: Nueva cita could open before a delayed write lands. */
export async function handOff(
	form: Form,
	replace: boolean
): Promise<'needs_confirm' | 'ok' | 'failed'> {
	const key = draftKey('cita', 'new');
	try {
		// A draft that fails its schema reads as none.
		if (!replace && (await getDraft(key, FormSchema)) !== null) {
			return 'needs_confirm';
		}
		await putDraft(key, FormSchema.parse(form));
		return 'ok';
	} catch {
		console.warn('code=cita_handoff_failed');
		return 'failed';
	}
}
