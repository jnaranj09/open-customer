// The phone cache: the one module that touches Dexie (guard `layers`). It is a read-only copy of
// what `/bootstrap` last said, plus drafts and the pending send confirmation. Every row read back
// is parsed once with its zod schema, so a bad cached row never reaches a screen
// (rules/boundaries.md). A bad snapshot or meta row clears the whole cache and reads as empty; a
// bad draft is deleted alone, because it is the app's own local value, not a copy of the server.
import Dexie, { type Table } from 'dexie';
import { z } from 'zod';
import { byPosition } from '../order';
import {
	AppointmentSchema,
	AttributeSchema,
	ClientSchema,
	DueItemSchema,
	MeSchema,
	RuleSchema,
	ServiceSchema,
	SettingsSchema,
	TemplateSchema,
	TemplateVarSchema,
	type Bootstrap,
	type Settings
} from '../api/schemas';

/** The Dexie version of the stores below. There is no upgrade step: a version bump keeps the old
 *  rows, and an old row is dropped only when it fails its schema on read (see `guarded`). */
export const SCHEMA_VERSION = 1;
export const DB_NAME = 'open-customer';

const PendingSendSchema = z.object({
	item_id: z.string(),
	client_name: z.string(),
	started_at: z.iso.datetime({ offset: true })
});

const MetaSchema = z.object({
	lastSyncAt: z.nullable(z.iso.datetime({ offset: true })),
	me: z.nullable(MeSchema),
	pending: z.nullable(PendingSendSchema),
	templateVars: z.array(TemplateVarSchema),
	backupRed: z.boolean()
});
export type Meta = z.infer<typeof MetaSchema>;
/** The send confirmation stored when she taps "Enviar" (design doc "Sending flow"). */
export type PendingSend = z.infer<typeof PendingSendSchema>;

/** The meta of a phone that never synced. */
export const EMPTY_META: Meta = {
	lastSyncAt: null,
	me: null,
	pending: null,
	templateVars: [],
	backupRed: false
};

/** What a refresh replaces: every table of `/bootstrap` that a screen reads. */
export type CacheSnapshot = Pick<
	Bootstrap,
	| 'settings'
	| 'attributes'
	| 'services'
	| 'templates'
	| 'rules'
	| 'clients'
	| 'appointments'
	| 'due'
>;

type Keyed = { key: string; value: unknown };
type DraftRow = { key: string; value: unknown };

const db = new Dexie(DB_NAME);
db.version(SCHEMA_VERSION).stores({
	settings: 'key',
	attributes: 'id',
	services: 'id',
	templates: 'id',
	rules: 'id',
	clients: 'id',
	appointments: 'id',
	due: 'id',
	drafts: 'key',
	meta: 'key'
});

const t = {
	settings: db.table<Keyed, string>('settings'),
	attributes: db.table<unknown, string>('attributes'),
	services: db.table<unknown, string>('services'),
	templates: db.table<unknown, string>('templates'),
	rules: db.table<unknown, string>('rules'),
	clients: db.table<unknown, string>('clients'),
	appointments: db.table<unknown, string>('appointments'),
	due: db.table<unknown, string>('due'),
	drafts: db.table<DraftRow, string>('drafts'),
	meta: db.table<Keyed, string>('meta')
};
const ALL_TABLES: Table[] = Object.values(t);

const SINGLE = 'only';

/** Deletes the whole database (cached data, due items, drafts, the pending confirmation) and
 *  opens an empty one. The 401 handler (`lib/api/app.ts`), the bad-row path below and
 *  logout (`lib/logout.ts`) call it (design doc item 67). */
export async function clearCache(): Promise<void> {
	try {
		await db.delete();
		await db.open();
	} finally {
		// Whatever the database did, what is held in memory is from the old session: drop it too.
		for (const listener of clearListeners) {
			listener();
		}
	}
}

const clearListeners = new Set<() => void>();

/** Runs `listener` each time the cache is cleared, so the in-memory read model drops its copy. */
export function onCacheCleared(listener: () => void): void {
	clearListeners.add(listener);
}

const badRowListeners = new Set<() => void>();

/** Runs `listener` after a bad snapshot or meta row has cleared the cache, so the store can ask
 *  `/bootstrap` again. A 401 or a logout clears the cache too, but does not call it. */
export function onBadRow(listener: () => void): void {
	badRowListeners.add(listener);
}

/** Thrown inside this module when a stored row fails its schema; `table` is only a table name. */
class BadRow extends Error {
	constructor(readonly table: string) {
		super('cache row invalid');
	}
}

function parseRow<T>(table: string, schema: z.ZodType<T>, raw: unknown): T {
	const parsed = schema.safeParse(raw);
	if (!parsed.success) {
		throw new BadRow(table);
	}
	return parsed.data;
}

const parseRows = <T>(table: string, schema: z.ZodType<T>, raw: unknown[]): T[] =>
	raw.map((row) => parseRow(table, schema, row));

/** Runs a read; a row that fails its schema logs the table and a code (never the row), clears the
 *  cache, tells the bad-row listeners and gives `onBad` instead. */
async function guarded<T>(read: () => Promise<T>, onBad: T): Promise<T> {
	try {
		return await read();
	} catch (e) {
		if (!(e instanceof BadRow)) {
			throw e;
		}
		console.warn(`code=cache_row_invalid table=${e.table}`);
		await clearCache();
		for (const listener of badRowListeners) {
			listener();
		}
		return onBad;
	}
}

/** The cached settings, or null before the first refresh. */
export function readSettings(): Promise<Settings | null> {
	return guarded(async () => {
		const row = await t.settings.get(SINGLE);
		return row === undefined ? null : parseRow('settings', SettingsSchema, row.value);
	}, null);
}

/** The cached snapshot, or null when there is none yet or a row failed its schema (the cache is
 *  then already cleared, and the caller asks for a refresh). */
export function readAll(): Promise<CacheSnapshot | null> {
	return guarded(async () => {
		const settings = await readSettings();
		if (settings === null) {
			return null;
		}
		const [attributes, services, templates, rules, clients, appointments, due] =
			await db.transaction('r', ALL_TABLES, () =>
				Promise.all([
					t.attributes.toArray(),
					t.services.toArray(),
					t.templates.toArray(),
					t.rules.toArray(),
					t.clients.toArray(),
					t.appointments.toArray(),
					t.due.toArray()
				])
			);
		return {
			settings,
			attributes: byPosition(parseRows('attributes', AttributeSchema, attributes), (a) => a.label),
			services: byPosition(parseRows('services', ServiceSchema, services), (x) => x.name),
			templates: parseRows('templates', TemplateSchema, templates),
			rules: parseRows('rules', RuleSchema, rules),
			clients: parseRows('clients', ClientSchema, clients),
			appointments: parseRows('appointments', AppointmentSchema, appointments),
			due: parseRows('due', DueItemSchema, due)
		};
	}, null);
}

export function readMeta(): Promise<Meta> {
	return guarded(async () => {
		const row = await t.meta.get(SINGLE);
		return row === undefined ? EMPTY_META : parseRow('meta', MetaSchema, row.value);
	}, EMPTY_META);
}

/** Merges `patch` into the stored meta. */
export async function writeMeta(patch: Partial<Meta>): Promise<void> {
	await db.transaction('rw', t.meta, async () => {
		const current = await readMetaRaw();
		await t.meta.put({ key: SINGLE, value: { ...current, ...patch } });
	});
}

async function readMetaRaw(): Promise<Meta> {
	const row = await t.meta.get(SINGLE);
	const parsed = MetaSchema.safeParse(row?.value);
	return parsed.success ? parsed.data : EMPTY_META;
}

/** Replaces every table with `data` and stores `syncedAt`, the template variables and the backup
 *  flag, all in one transaction: a failure leaves the old data in place. The pending
 *  confirmation and the last `Me` in meta are kept. */
export async function writeSnapshot(data: Bootstrap, syncedAt: string): Promise<void> {
	await db.transaction('rw', ALL_TABLES, async () => {
		const current = await readMetaRaw();
		const replace = async (table: Table<unknown, string>, items: unknown[]): Promise<void> => {
			await table.clear();
			await table.bulkPut(items);
		};
		await t.settings.put({ key: SINGLE, value: data.settings });
		await replace(t.attributes, data.attributes);
		await replace(t.services, data.services);
		await replace(t.templates, data.templates);
		await replace(t.rules, data.rules);
		await replace(t.clients, data.clients);
		await replace(t.appointments, data.appointments);
		await replace(t.due, data.due);
		const meta: Meta = {
			...current,
			lastSyncAt: syncedAt,
			templateVars: data.template_vars,
			backupRed: data.backup_red
		};
		await t.meta.put({ key: SINGLE, value: meta });
	});
}

/** Stores a draft under its key (see `drafts.ts`). */
export async function putDraft(key: string, value: unknown): Promise<void> {
	await t.drafts.put({ key, value });
}

/** The stored draft value, parsed with `schema`; null when absent. A draft that fails its schema
 *  (one from an older form shape) is deleted and reads null; the rest of the cache stays, the
 *  pending send confirmation included. */
export async function getDraft<T>(key: string, schema: z.ZodType<T>): Promise<T | null> {
	const row = await t.drafts.get(key);
	if (row === undefined) {
		return null;
	}
	const parsed = schema.safeParse(row.value);
	if (parsed.success) {
		return parsed.data;
	}
	console.warn('code=draft_invalid');
	await t.drafts.delete(key);
	return null;
}

export async function deleteDraft(key: string): Promise<void> {
	await t.drafts.delete(key);
}
