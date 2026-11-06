// E2E test data (design doc item 80): plain SQL through `pg` against the e2e database, outside the
// app. The API always sets `created_at` to now, so a spec that needs a visit in the past inserts it
// here. Every spec except `setup.spec.ts` calls `resetData` before each test and `closePool` after
// all of them. Nothing in the release binary changes for this.
import { randomUUID } from 'node:crypto';
import pg from 'pg';
import { ZONE, databaseUrl } from './env';

// One pool per Playwright worker: the module is loaded once per worker.
let pool: pg.Pool | undefined;

function db(): pg.Pool {
	pool ??= new pg.Pool({ connectionString: databaseUrl() });
	return pool;
}

/** Empties the per-spec tables. The user, sessions, settings, attributes, services, templates and
 *  rules stay, so the saved login and the setup data keep working. */
export async function resetData(): Promise<void> {
	await db().query(
		'TRUNCATE clients, appointments, appointment_services, media, reminder_log CASCADE'
	);
}

/** Ends the worker's pool; `afterAll` of each spec that seeds. */
export async function closePool(): Promise<void> {
	await pool?.end();
	pool = undefined;
}

const DAY_MS = 24 * 60 * 60 * 1000;

/** The business day `dayOffset` days from today in `ZONE`, as YYYY-MM-DD. */
export function businessDate(dayOffset: number): string {
	// en-CA formats a date as YYYY-MM-DD.
	const today = new Intl.DateTimeFormat('en-CA', {
		timeZone: ZONE,
		year: 'numeric',
		month: '2-digit',
		day: '2-digit'
	}).format(new Date());
	return new Date(Date.parse(`${today}T00:00:00Z`) + dayOffset * DAY_MS).toISOString().slice(0, 10);
}

/** `hhmm` on the business day `dayOffset` days from today. `ZONE` has no DST: always -06:00. */
export function at(dayOffset: number, hhmm: string): string {
	return `${businessDate(dayOffset)}T${hhmm}:00-06:00`;
}

/** The key of the active client attribute labelled `label`; the keys are made by the app. */
async function attrKey(label: string): Promise<string> {
	const { rows } = await db().query<{ key: string }>(
		"SELECT key FROM attribute_definitions WHERE label = $1 AND scope = 'client' AND archived_at IS NULL",
		[label]
	);
	const row = rows[0];
	if (row === undefined) {
		throw new Error(`seed: no client attribute "${label}"`);
	}
	return row.key;
}

/** Inserts an active client and returns its id. `phone` is E.164 ("+50688887777"); `attrs` maps
 *  attribute labels to their stored values. */
export async function insertClient(c: {
	name: string;
	phone?: string;
	attrs?: Record<string, string>;
}): Promise<string> {
	const attrs: Record<string, string> = {};
	for (const [label, value] of Object.entries(c.attrs ?? {})) {
		attrs[await attrKey(label)] = value;
	}
	const id = randomUUID();
	await db().query(
		'INSERT INTO clients (id, name, phone_e164, phone_digits, attrs) VALUES ($1, $2, $3, $4, $5)',
		[id, c.name, c.phone ?? null, c.phone?.replace(/^\+/, '') ?? null, attrs]
	);
	return id;
}

type ServiceRow = { id: string; name: string; price_cents: string | null; duration_min: number };

/** Inserts an appointment of `clientId` with the active services named in `services` (in order;
 *  a name may repeat), their name, price and duration copied in. It ends after `minutes`, by
 *  default the sum of the durations, and costs the sum of the prices. Returns its id. */
export async function insertAppointment(a: {
	clientId: string;
	services: string[];
	startsAt: string;
	createdAt?: string;
	status?: 'scheduled' | 'cancelled' | 'no_show';
	minutes?: number;
}): Promise<string> {
	const found = await db().query<ServiceRow>(
		'SELECT id, name, price_cents, duration_min FROM services WHERE name = ANY($1) AND archived_at IS NULL',
		[a.services]
	);
	const byName = new Map(found.rows.map((s) => [s.name, s]));
	const lines = a.services.map((name) => {
		const s = byName.get(name);
		if (s === undefined) {
			throw new Error(`seed: no active service "${name}"`);
		}
		return s;
	});
	const minutes = a.minutes ?? lines.reduce((sum, s) => sum + s.duration_min, 0);
	// bigint arrives as a string.
	const total = lines.reduce((sum, s) => sum + Number(s.price_cents ?? 0), 0);
	const startsMs = Date.parse(a.startsAt);
	if (Number.isNaN(startsMs) || minutes === 0) {
		throw new Error('seed: an appointment needs a start time and at least one service');
	}
	const endsAt = new Date(startsMs + minutes * 60_000).toISOString();
	const createdAt = a.createdAt ?? new Date().toISOString();

	const id = randomUUID();
	const conn = await db().connect();
	try {
		await conn.query('BEGIN');
		await conn.query(
			`INSERT INTO appointments (id, client_id, starts_at, ends_at, status, total_cents, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $7)`,
			[id, a.clientId, a.startsAt, endsAt, a.status ?? 'scheduled', total, createdAt]
		);
		for (const [position, s] of lines.entries()) {
			await conn.query(
				`INSERT INTO appointment_services (appointment_id, position, service_id, name, price_cents, duration_min)
				 VALUES ($1, $2, $3, $4, $5, $6)`,
				[id, position, s.id, s.name, s.price_cents, s.duration_min]
			);
		}
		await conn.query('COMMIT');
	} catch (e) {
		await conn.query('ROLLBACK');
		throw e;
	} finally {
		conn.release();
	}
	return id;
}
