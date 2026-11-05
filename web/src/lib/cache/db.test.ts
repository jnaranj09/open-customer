import 'fake-indexeddb/auto';
import Dexie from 'dexie';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { z } from 'zod';
import {
	DB_NAME,
	SCHEMA_VERSION,
	clearCache,
	getDraft,
	putDraft,
	readAll,
	readMeta,
	writeMeta,
	writeSnapshot
} from './db';
import { bootstrap } from './fixtures';

const SYNCED = '2026-11-04T12:00:01Z';
const PENDING = { item_id: 'a:1', client_name: 'Ana Rojas', started_at: '2026-11-04T12:05:00Z' };

/** A second connection to the same database, to break or count stored rows. */
async function raw<T>(use: (db: Dexie) => Promise<T>): Promise<T> {
	const db = new Dexie(DB_NAME);
	await db.open();
	try {
		return await use(db);
	} finally {
		db.close();
	}
}

beforeEach(async () => {
	await clearCache();
});

describe('reading the cache', () => {
	it('reads null before the first refresh', async () => {
		expect(await readAll()).toBeNull();
	});

	it('reads back what a refresh wrote', async () => {
		await writeSnapshot(bootstrap(2), SYNCED);
		const snapshot = await readAll();
		expect(snapshot?.clients.map((c) => c.name)).toEqual(['Ana Rojas']);
		expect(snapshot?.due).toHaveLength(2);
	});

	it('a cached row that fails its schema clears the cache and reads null', async () => {
		vi.spyOn(console, 'warn').mockImplementation(() => undefined);
		await writeSnapshot(bootstrap(1), SYNCED);
		await raw((db) => db.table('clients').toCollection().modify({ name: 42 }));
		expect(await readAll()).toBeNull();
		expect(await raw((db) => db.table('clients').count())).toBe(0);
	});

	it('logs the table and a code, never the row', async () => {
		const warn = vi.spyOn(console, 'warn').mockImplementation(() => undefined);
		await writeSnapshot(bootstrap(1, 'Marta Solano'), SYNCED);
		await raw((db) => db.table('clients').toCollection().modify({ name: 42 }));
		await readAll();
		expect(warn).toHaveBeenCalledWith('code=cache_row_invalid table=clients');
	});
});

describe('replacing the cache', () => {
	it('replaces every table, so a client that left the answer is gone', async () => {
		await writeSnapshot(bootstrap(3, 'Ana Rojas'), SYNCED);
		await writeSnapshot(bootstrap(0, 'Lucía Rojas'), SYNCED);
		const snapshot = await readAll();
		expect(snapshot?.clients.map((c) => c.name)).toEqual(['Lucía Rojas']);
		expect(snapshot?.due).toHaveLength(0);
	});

	it('a failure mid-write leaves the old data', async () => {
		await writeSnapshot(bootstrap(2, 'Ana Rojas'), SYNCED);
		const broken = bootstrap(0, 'Lucía Rojas');
		// A function cannot be stored in IndexedDB: the client write fails after the clears ran.
		broken.clients = broken.clients.map((c) => ({ ...c, attrs: { x: () => 1 } }));
		await expect(writeSnapshot(broken, SYNCED)).rejects.toThrow();
		const snapshot = await readAll();
		expect(snapshot?.clients.map((c) => c.name)).toEqual(['Ana Rojas']);
		expect(snapshot?.due).toHaveLength(2);
	});

	it('stores lastSyncAt, the template variables and the backup flag with the tables', async () => {
		await writeSnapshot({ ...bootstrap(0), backup_red: true }, SYNCED);
		const meta = await readMeta();
		expect(meta.lastSyncAt).toBe(SYNCED);
		expect(meta.backupRed).toBe(true);
		expect(meta.templateVars).toEqual([{ ns: 'client', key: 'name', label: 'Nombre' }]);
	});

	it('keeps the pending confirmation and the last Me across a refresh', async () => {
		await writeMeta({ pending: PENDING });
		await writeSnapshot(bootstrap(0), SYNCED);
		expect((await readMeta()).pending).toEqual(PENDING);
	});
});

describe('meta', () => {
	it('starts empty', async () => {
		expect(await readMeta()).toEqual({
			lastSyncAt: null,
			me: null,
			pending: null,
			templateVars: [],
			backupRed: false
		});
	});

	it('merges a patch into what is stored', async () => {
		await writeMeta({ backupRed: true });
		await writeMeta({ pending: PENDING });
		const meta = await readMeta();
		expect(meta.backupRed).toBe(true);
		expect(meta.pending).toEqual(PENDING);
	});

	it('a meta row that fails its schema clears the cache and reads the empty meta', async () => {
		vi.spyOn(console, 'warn').mockImplementation(() => undefined);
		await writeSnapshot(bootstrap(1), SYNCED);
		await raw((db) =>
			db
				.table('meta')
				.toCollection()
				.modify({ value: { lastSyncAt: 5 } })
		);
		expect((await readMeta()).lastSyncAt).toBeNull();
		expect(await raw((db) => db.table('clients').count())).toBe(0);
	});
});

describe('the database', () => {
	it('opens at the schema version', async () => {
		expect(await raw((db) => Promise.resolve(db.verno))).toBe(SCHEMA_VERSION);
	});
});

describe('clearCache', () => {
	it('removes drafts and the pending confirmation', async () => {
		await putDraft('cita:new', { a: 1 });
		await writeMeta({ pending: PENDING });
		await clearCache();
		expect(await getDraft('cita:new', z.unknown())).toBeNull();
		expect((await readMeta()).pending).toBeNull();
	});

	it('removes the cached tables', async () => {
		await writeSnapshot(bootstrap(1), SYNCED);
		await clearCache();
		expect(await readAll()).toBeNull();
	});
});
