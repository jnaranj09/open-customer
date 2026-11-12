// The pure rules of the appointment form (design doc item 75): the service lines, the end time,
// the option drill-down, the request bodies and the form's own checks. The form component owns
// the state and the calls.
import { z } from 'zod';
import type { components } from './api/schema';
import type { Appointment, AttrOption, Attribute, Service } from './api/schemas';
import type { MessageKey } from './i18n';
import { clockText, formatDayLong, formatTime, todayIn, zonedToUtc } from './time';

type Schemas = components['schemas'];

/** A service on the form and how many copies of it she picked ("×2"). */
export const LineSchema = z.object({ service_id: z.string(), count: z.number().int().positive() });
export type Line = z.infer<typeof LineSchema>;

/** A tap on a service: the first adds it, a second adds a copy. New lines go last. */
export function tapService(lines: Line[], id: string): Line[] {
	return lines.some((l) => l.service_id === id)
		? lines.map((l) => (l.service_id === id ? { ...l, count: l.count + 1 } : l))
		: [...lines, { service_id: id, count: 1 }];
}

/** Takes one copy of a service away; the line goes with its last copy. */
export function removeService(lines: Line[], id: string): Line[] {
	return lines
		.map((l) => (l.service_id === id ? { ...l, count: l.count - 1 } : l))
		.filter((l) => l.count > 0);
}

/** The `service_ids` to send: each service once per copy, in line order. */
export function serviceIds(lines: Line[]): string[] {
	return lines.flatMap((l) => Array.from({ length: l.count }, () => l.service_id));
}

const validTime = (time: string): boolean => /^\d{2}:\d{2}$/.test(time);
/** Whether `day` is a real calendar day written `YYYY-MM-DD`. */
export const validDay = (day: string): boolean => z.iso.date().safeParse(day).success;

const MINUTES_PER_DAY = 24 * 60;

function minutesOf(time: string): number {
	const [h, m] = time.split(':');
	return Number(h) * 60 + Number(m);
}

function timeOf(minutes: number): string {
	const wrapped = ((minutes % MINUTES_PER_DAY) + MINUTES_PER_DAY) % MINUTES_PER_DAY;
	return clockText(Math.floor(wrapped / 60), wrapped % 60);
}

/** The end time (`HH:MM`) of a booking that starts at `startLocal`: the sum of the durations. A
 *  booking that runs past midnight shows the time on the next day. */
export function endFor(startLocal: string, lines: Line[], services: Service[]): string {
	if (!validTime(startLocal)) {
		return '';
	}
	return timeOf(minutesOf(startLocal) + durationOf(lines, services));
}

/** The minutes the lines take: each service's duration once per copy. A service not in the list
 *  adds 0. */
export function durationOf(lines: Line[], services: Service[]): number {
	return lines.reduce(
		(sum, l) => sum + l.count * (services.find((s) => s.id === l.service_id)?.duration_min ?? 0),
		0
	);
}

/** The ids of the services that are not archived. */
export function activeServiceIds(services: Service[]): Set<string> {
	return new Set(services.filter((s) => !s.archived).map((s) => s.id));
}

/** The sum of the prices; a service with no price counts zero. */
export function totalOf(lines: Line[], services: Service[]): number {
	return lines.reduce(
		(sum, l) => sum + l.count * (services.find((s) => s.id === l.service_id)?.price_cents ?? 0),
		0
	);
}

/** The chain from the top option down to `id` (the drill-down breadcrumb); empty for none. */
export function optionPath(options: AttrOption[], id: string | null): AttrOption[] {
	const path: AttrOption[] = [];
	let at = options.find((o) => o.id === id);
	while (at !== undefined && !path.includes(at)) {
		path.unshift(at);
		const parent: string | null = at.parent_id;
		at = options.find((o) => o.id === parent);
	}
	return path;
}

/** Where "save" goes back to: an app path she came from, or Hoy. Only a path inside this app
 *  passes (one slash first, no host, no backslash). */
export function returnTo(from: string | null): string {
	if (from === null) {
		return '/';
	}
	// The URL parser drops tabs and newlines and reads a backslash as a slash, so the check runs on
	// what it makes of the text, not on the text.
	try {
		const url = new URL(from, 'http://app.invalid');
		return url.origin === 'http://app.invalid' && from.startsWith('/') && !from.startsWith('//')
			? url.pathname + url.search
			: '/';
	} catch {
		console.warn('code=return_path_invalid');
		return '/';
	}
}

/** The `dia` of the link that opened the form, or `today` when it is missing or not a real day. */
export function dayParam(raw: string | null, today: string): string {
	return raw !== null && z.iso.date().safeParse(raw).success ? raw : today;
}

/** The services a form prices and times with. On an appointment already booked, a service on it
 *  keeps its saved duration and price (the server keeps the saved line), and one archived since
 *  stays in the list so she can still see and remove it. */
export function withSavedLines(services: Service[], saved: Appointment['services']): Service[] {
	const out = services.map((s) => {
		const line = saved.find((l) => l.service_id === s.id);
		return line === undefined
			? s
			: { ...s, duration_min: line.duration_min, price_cents: line.price_cents };
	});
	for (const line of saved) {
		if (!out.some((s) => s.id === line.service_id)) {
			out.push({
				id: line.service_id,
				name: line.name,
				description: null,
				duration_min: line.duration_min,
				price_cents: line.price_cents,
				color: '',
				position: 0,
				archived: true
			});
		}
	}
	return out;
}

const AttrValueSchema = z.union([z.string(), z.array(z.string())]);
export type AttrValue = z.infer<typeof AttrValueSchema>;

/** The form's values. `end` is null until she sets the end time by hand; until then it follows the
 *  services. A number is kept as the text she typed. The draft stores this shape. */
export const FormSchema = z.object({
	clientId: z.nullable(z.string()),
	lines: z.array(LineSchema),
	day: z.string(),
	time: z.string(),
	end: z.nullable(z.string()),
	notes: z.string(),
	attrs: z.record(z.string(), AttrValueSchema)
});
export type Form = z.infer<typeof FormSchema>;

/** The form of a new appointment on `day`: nothing picked yet. */
export function blankForm(day: string): Form {
	return { clientId: null, lines: [], day, time: '', end: null, notes: '', attrs: {} };
}

export type FormErrors = Partial<Record<'client' | 'day' | 'time' | 'end', MessageKey>>;

/** The checks that need no server: who, when, and an end time that makes sense. With no services
 *  the end time is needed on a new booking, or when an edit takes the last service away
 *  (`hadServices`); a notes-only edit of a booking that never had services needs none. */
export function formErrors(form: Form, creating: boolean, hadServices = false): FormErrors {
	const errors: FormErrors = {};
	if (creating && form.clientId === null) {
		errors.client = 'appointment_client_needed';
	}
	if (!validDay(form.day)) {
		errors.day = 'appointment_day_needed';
	}
	if (!validTime(form.time)) {
		errors.time = 'appointment_time_needed';
	} else if (form.end === null && form.lines.length === 0 && (creating || hadServices)) {
		errors.end = 'appointment_end_needed';
	} else if (form.end !== null && minutesOf(form.end) <= minutesOf(form.time)) {
		errors.end = 'appointment_end_after';
	}
	return errors;
}

const isEmpty = (v: AttrValue | undefined): boolean =>
	v === undefined || (typeof v === 'string' ? v.trim() === '' : v.length === 0);

const NUMBER = /^-?\d+([.,]\d+)?$/;

/** One value as the API takes it. A number typed with a comma is read as a number; text that is no
 *  number goes as typed, and the server says why it is wrong. */
function sendable(def: Attribute, v: AttrValue): unknown {
	if (typeof v !== 'string') {
		return v;
	}
	const text = v.trim();
	return def.type === 'number' && NUMBER.test(text) ? Number(text.replace(',', '.')) : text;
}

/** The attribute values to send for a new record: the filled ones, each as the API takes it. */
export function attrsToSend(
	values: Record<string, AttrValue>,
	defs: Attribute[]
): Record<string, unknown> {
	const out: Record<string, unknown> = {};
	for (const def of defs) {
		const v = values[def.key];
		if (v !== undefined && !isEmpty(v)) {
			out[def.key] = sendable(def, v);
		}
	}
	return out;
}

/** The body of `POST /appointments`. The end time goes only when she set it; otherwise the server
 *  adds up the durations. */
export function createBody(
	form: Form,
	zone: string,
	defs: Attribute[]
): Schemas['AppointmentInput'] {
	const attrs = attrsToSend(form.attrs, defs);
	const notes = form.notes.trim();
	return {
		client_id: form.clientId ?? '',
		starts_at: zonedToUtc(form.day, form.time, zone),
		...(form.end === null ? {} : { ends_at: zonedToUtc(form.day, form.end, zone) }),
		...(form.lines.length === 0 ? {} : { service_ids: serviceIds(form.lines) }),
		...(Object.keys(attrs).length === 0 ? {} : { attrs }),
		...(notes === '' ? {} : { notes })
	};
}

const sameInstant = (a: string, b: string): boolean => Date.parse(a) === Date.parse(b);

const sameList = (a: string[], b: string[]): boolean =>
	a.length === b.length && a.every((v, i) => v === b[i]);

function textOf(v: unknown): AttrValue | undefined {
	if (typeof v === 'string' || typeof v === 'number') {
		return String(v);
	}
	return Array.isArray(v) ? v.filter((x): x is string => typeof x === 'string') : undefined;
}

/** The lines of a booked appointment: one per service, in the order it first appears, with its
 *  copies counted (the appointment stores one entry per copy). */
export function linesOf(saved: Appointment['services']): Line[] {
	const lines: Line[] = [];
	for (const s of saved) {
		const at = lines.find((l) => l.service_id === s.service_id);
		if (at === undefined) {
			lines.push({ service_id: s.service_id, count: 1 });
		} else {
			at.count += 1;
		}
	}
	return lines;
}

/** The form for an appointment already booked. */
export function formFrom(a: Appointment, zone: string, defs: Attribute[]): Form {
	return {
		clientId: a.client_id,
		lines: linesOf(a.services),
		day: todayIn(zone, new Date(a.starts_at)),
		time: formatTime(a.starts_at, zone),
		end: null,
		notes: a.notes ?? '',
		attrs: attrFormValues(a.attrs, defs)
	};
}

/** The form values of a record's stored attributes (only the defined ones that hold a value). */
export function attrFormValues(
	stored: Record<string, unknown>,
	defs: Attribute[]
): Record<string, AttrValue> {
	const attrs: Record<string, AttrValue> = {};
	for (const def of defs) {
		const v = textOf(stored[def.key]);
		if (v !== undefined) {
			attrs[def.key] = v;
		}
	}
	return attrs;
}

/** What changed in a record's attributes: the new or changed values, `null` for a cleared one;
 *  null when nothing changed. */
export function attrChanges(
	before: Record<string, unknown>,
	values: Record<string, AttrValue>,
	defs: Attribute[]
): Record<string, unknown> | null {
	const out: Record<string, unknown> = {};
	for (const def of defs) {
		const was = textOf(before[def.key]);
		const now = values[def.key];
		if (isEmpty(now)) {
			if (!isEmpty(was)) {
				out[def.key] = null;
			}
		} else if (now !== undefined && JSON.stringify(now) !== JSON.stringify(was)) {
			out[def.key] = sendable(def, now);
		}
	}
	return Object.keys(out).length === 0 ? null : out;
}

/** The body of `PATCH /appointments/{id}`: only what changed. With no `ends_at` the server moves
 *  the end with the start and adds up the new durations, so it goes only when she set it. */
export function patchBody(
	before: Appointment,
	form: Form,
	zone: string,
	defs: Attribute[]
): Schemas['AppointmentPatch'] {
	const body: Schemas['AppointmentPatch'] = {};
	const starts = zonedToUtc(form.day, form.time, zone);
	if (!sameInstant(starts, before.starts_at)) {
		body.starts_at = starts;
	}
	if (form.end !== null) {
		const ends = zonedToUtc(form.day, form.end, zone);
		if (!sameInstant(ends, before.ends_at) || body.starts_at !== undefined) {
			body.ends_at = ends;
		}
	}
	const ids = serviceIds(form.lines);
	if (
		!sameList(
			ids,
			before.services.map((s) => s.service_id)
		)
	) {
		body.service_ids = ids;
	}
	const attrs = attrChanges(before.attrs, form.attrs, defs);
	if (attrs !== null) {
		body.attrs = attrs;
	}
	const notes = form.notes.trim();
	if (notes !== (before.notes ?? '')) {
		body.notes = notes === '' ? null : notes;
	}
	return body;
}

/** The end time the form shows. One she set wins. Otherwise it is what the server will save: an
 *  appointment whose services did not change keeps its length and moves with the start; any other
 *  adds up the durations. */
export function endShown(
	form: Form,
	before: Appointment | null,
	zone: string,
	services: Service[]
): string {
	if (form.end !== null) {
		return form.end;
	}
	if (
		before !== null &&
		validDay(form.day) &&
		validTime(form.time) &&
		sameList(
			serviceIds(form.lines),
			before.services.map((s) => s.service_id)
		)
	) {
		const shift = Date.parse(zonedToUtc(form.day, form.time, zone)) - Date.parse(before.starts_at);
		return formatTime(new Date(Date.parse(before.ends_at) + shift).toISOString(), zone);
	}
	return endFor(form.time, form.lines, services);
}

/** A saved draft with what no longer exists taken out: a client, service or attribute removed or
 *  archived since she wrote it must not reach the request. */
export function restoreForm(
	saved: Form,
	known: { clients: Set<string>; services: Set<string>; attrKeys: Set<string> }
): Form {
	return {
		...saved,
		clientId: saved.clientId !== null && known.clients.has(saved.clientId) ? saved.clientId : null,
		lines: saved.lines.filter((l) => known.services.has(l.service_id)),
		attrs: Object.fromEntries(
			Object.entries(saved.attrs).filter(([key]) => known.attrKeys.has(key))
		)
	};
}

const STATUS_ACTIONS: Record<Appointment['status'], Appointment['status'][]> = {
	scheduled: ['confirmed', 'cancelled', 'no_show'],
	confirmed: ['cancelled', 'no_show'],
	cancelled: ['scheduled'],
	no_show: ['scheduled']
};

/** The name of each status as the screens show it. */
export const STATUS_NAME: Record<Appointment['status'], MessageKey> = {
	scheduled: 'status_scheduled',
	confirmed: 'status_confirmed',
	cancelled: 'status_cancelled',
	no_show: 'status_no_show'
};

/** The statuses she can move an appointment to (the table in "Appointments"): "Reactivar" is the
 *  move back to scheduled. The server still checks every change. */
export function statusActions(status: Appointment['status']): Appointment['status'][] {
	return STATUS_ACTIONS[status];
}

/** A stored attribute value as text for the detail screen, or null when there is none. */
export function attrDisplay(def: Attribute, raw: unknown): string | null {
	const v = textOf(raw);
	if (v === undefined || isEmpty(v)) {
		return null;
	}
	if (def.type === 'select' || def.type === 'multiselect') {
		const ids = typeof v === 'string' ? [v] : v;
		return ids
			.map((id) =>
				optionPath(def.options, id)
					.map((o) => o.label)
					.join(' ')
			)
			.join(', ');
	}
	return def.type === 'date' && typeof v === 'string' ? formatDayLong(v) : String(v);
}
