// The pure part of Enviar → Respuestas (reminders-and-replies design, "Quick replies").
import type { QuickReply } from './api/schemas';
import { normalize } from './search';

/** The replies whose title or body holds `query`, ignoring case and accents, in her order. A blank
 *  query keeps them all. */
export function filterReplies(list: readonly QuickReply[], query: string): QuickReply[] {
	const q = normalize(query).trim();
	if (q === '') {
		return [...list];
	}
	return list.filter((r) => normalize(r.title).includes(q) || normalize(r.body).includes(q));
}
