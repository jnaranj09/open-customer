// The Dexie upgrade from version 1, in its own file: the cache module opens its database on first
// use, so the version-1 database must exist before the module is imported.
import 'fake-indexeddb/auto';
import Dexie from 'dexie';
import { describe, expect, it } from 'vitest';
import { z } from 'zod';
import { bootstrap } from './fixtures';

const DB_NAME = 'open-customer';
const PENDING = { item_id: 'a:1', client_name: 'Ana Rojas', started_at: '2026-11-04T12:05:00Z' };
const V1_STORES = {
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

describe('upgrading the cache from version 1', () => {
	it('clears the copied server data and keeps the pending confirmation and drafts', async () => {
		const data = bootstrap(1);
		const v1 = new Dexie(DB_NAME);
		v1.version(1).stores(V1_STORES);
		await v1.open();
		await v1.table('settings').put({ key: 'only', value: data.settings });
		// A version-1 due row: the bare item, no position.
		await v1.table('due').bulkPut(data.due);
		await v1.table('clients').bulkPut(data.clients);
		await v1.table('drafts').put({ key: 'clienta:new', value: { name: 'Lucía Rojas' } });
		await v1.table('meta').put({
			key: 'only',
			value: {
				lastSyncAt: '2026-11-04T12:00:01Z',
				me: null,
				pending: PENDING,
				templateVars: [],
				backupRed: false,
				logged: ['a:1']
			}
		});
		v1.close();

		const cache = await import('./db');
		// No snapshot: the store reads null and refreshes from /bootstrap.
		expect(await cache.readAll()).toBeNull();
		const meta = await cache.readMeta();
		expect(meta.pending).toEqual(PENDING);
		expect(meta.logged).toEqual(['a:1']);
		expect(await cache.getDraft('clienta:new', z.object({ name: z.string() }))).toEqual({
			name: 'Lucía Rojas'
		});
	});
});
