// Test seam for the cache upgrade tests (db.upgrade-v*.test.ts): an older version of the database
// with one row of copied data, a draft and a pending confirmation, written before the cache module
// is imported. Each test file then imports './db' and checks what the upgrade kept.
import 'fake-indexeddb/auto';
import Dexie from 'dexie';
import { bootstrap } from './fixtures';

export const PENDING = {
	item_id: 'a:1',
	client_name: 'Ana Rojas',
	started_at: '2026-11-04T12:05:00Z'
};

const STORES = {
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
};

/** Writes a database at `version` holding `row` (a row of the old shape) in `table`. */
export async function seedOldCache(version: number, table: string, row: object): Promise<void> {
	const old = new Dexie('open-customer');
	old.version(version).stores(STORES);
	await old.open();
	await old.table('settings').put({ key: 'only', value: bootstrap(1).settings });
	await old.table(table).put(row);
	await old.table('drafts').put({ key: 'clienta:new', value: { name: 'Lucía Rojas' } });
	await old.table('meta').put({
		key: 'only',
		value: {
			lastSyncAt: '2026-11-04T12:00:01Z',
			me: null,
			pending: PENDING,
			templateVars: [],
			backupRed: false,
			logged: []
		}
	});
	old.close();
}
