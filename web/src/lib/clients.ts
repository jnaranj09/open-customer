// The pure rules of the client screens (design doc item 76): the list a search box shows, the
// split of a client's history around the cached window, and the client form's checks and request
// bodies. The components own the state and the calls.
import { z } from 'zod';
import { inWindow } from './agenda';
import type { ApiError } from './api/errors';
import type { components } from './api/schema';
import type { Appointment, Attribute, Client } from './api/schemas';
import { attrChanges, attrFormValues, attrsToSend, returnTo } from './booking';
import type { MessageKey } from './i18n';
import { search, sortByName } from './search';
import { todayIn } from './time';

type Schemas = components['schemas'];

/** What the list shows: every client, by name, while the box is empty; the search result (at most
 *  20, best match first) otherwise. */
export function visibleClients(clients: readonly Client[], query: string): Client[] {
	return query.trim() === '' ? sortByName(clients) : search(clients, query);
}

/** A client's appointments split around the cached window (`bootstrapWindow`): `cached` are
 *  inside it (both end days included), `needsServer` are before or after it and come from the
 *  server only. The day is the business day of each start. */
export function historySplit(
	appts: readonly Appointment[],
	window: { from: string; to: string },
	zone: string
): { cached: Appointment[]; needsServer: Appointment[] } {
	const cached: Appointment[] = [];
	const needsServer: Appointment[] = [];
	for (const a of appts) {
		const day = todayIn(zone, new Date(a.starts_at));
		(inWindow(day, window) ? cached : needsServer).push(a);
	}
	return { cached, needsServer };
}

/** The names of the clients that share the phone, when the error is a `duplicate-phone` 409. */
export function sharingNames(error: ApiError | null): string[] | null {
	return error?.kind === 'problem' && error.type === 'duplicate-phone'
		? (error.problem.clients ?? []).map((c) => c.name)
		: null;
}

const AttrValueSchema = z.union([z.string(), z.array(z.string())]);

/** The client form's values; the draft stores this shape. A number is kept as typed text. */
export const ClientFormSchema = z.object({
	name: z.string(),
	phone: z.string(),
	notes: z.string(),
	attrs: z.record(z.string(), AttrValueSchema)
});
export type ClientForm = z.infer<typeof ClientFormSchema>;

export type ClientFormErrors = Partial<Record<'name', MessageKey>>;

/** The check that needs no server: a client needs a name. */
export function clientFormErrors(form: ClientForm): ClientFormErrors {
	return form.name.trim() === '' ? { name: 'client_name_needed' } : {};
}

/** The body of `POST /clients`: only what she filled, trimmed. */
export function clientCreateBody(form: ClientForm, defs: Attribute[]): Schemas['ClientInput'] {
	const phone = form.phone.trim();
	const notes = form.notes.trim();
	const attrs = attrsToSend(form.attrs, defs);
	return {
		name: form.name.trim(),
		...(phone === '' ? {} : { phone }),
		...(notes === '' ? {} : { notes }),
		...(Object.keys(attrs).length === 0 ? {} : { attrs })
	};
}

/** The form for a client already saved; attributes that are gone from `defs` are left out. */
export function clientFormFrom(c: Client, defs: Attribute[]): ClientForm {
	return {
		name: c.name,
		phone: c.phone_e164 ?? '',
		notes: c.notes ?? '',
		attrs: attrFormValues(c.attrs, defs)
	};
}

/** The body of `PATCH /clients/{id}`: only what changed. A cleared phone or notes goes as `null`,
 *  a cleared attribute as `null` under its key. */
export function clientPatchBody(
	before: Client,
	form: ClientForm,
	defs: Attribute[]
): Schemas['ClientPatch'] {
	const body: Schemas['ClientPatch'] = {};
	const name = form.name.trim();
	if (name !== before.name) {
		body.name = name;
	}
	const phone = form.phone.trim();
	if (phone !== (before.phone_e164 ?? '')) {
		body.phone = phone === '' ? null : phone;
	}
	const notes = form.notes.trim();
	if (notes !== (before.notes ?? '')) {
		body.notes = notes === '' ? null : notes;
	}
	const attrs = attrChanges(before.attrs, form.attrs, defs);
	if (attrs !== null) {
		body.attrs = attrs;
	}
	return body;
}

/** A saved draft without the attributes that no longer exist. */
export function restoreClientForm(saved: ClientForm, attrKeys: Set<string>): ClientForm {
	return {
		...saved,
		attrs: Object.fromEntries(Object.entries(saved.attrs).filter(([key]) => attrKeys.has(key)))
	};
}

/** Where a saved new client goes (design doc item 76). On compact, back to the screen she came
 *  from (`from` is the `volver` link value, checked by `returnTo`; the list when there is none).
 *  On wide the screen she came from is the list pane, so the new client is selected in it. */
export function savedDestination(from: string | null, wide: boolean, id: string): string {
	if (wide) {
		return `/clientas/${id}`;
	}
	return from === null ? '/clientas' : returnTo(from);
}
