// The Dexie upgrade from version 2 (services gained `description`), in its own file for the same
// reason as db.upgrade.test.ts: the version-2 database must exist before the module is imported.
import 'fake-indexeddb/auto';
import Dexie from 'dexie';
import { describe, expect, it } from 'vitest';
import { z } from 'zod';
import { bootstrap } from './fixtures';

const DB_NAME = 'open-customer';
const PENDING = { item_id: 'a:1', client_name: 'Ana Rojas', started_at: '2026-11-04T12:05:00Z' };
const V2_STORES = {
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

describe('upgrading the cache from version 2', () => {
	it('drops the cached services that lack a description, and keeps drafts and the pending confirmation', async () => {
		const data = bootstrap(1);
		const v2 = new Dexie(DB_NAME);
		v2.version(2).stores(V2_STORES);
		await v2.open();
		await v2.table('settings').put({ key: 'only', value: data.settings });
		// A version-2 service row: no description field.
		await v2.table('services').put({
			id: '11111111-1111-4111-8111-111111111111',
			name: 'Acrílico',
			duration_min: 90,
			price_cents: null,
			color: '#aa3366',
			position: 0,
			archived: false
		});
		await v2.table('drafts').put({ key: 'clienta:new', value: { name: 'Lucía Rojas' } });
		await v2.table('meta').put({
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
		v2.close();

		const cache = await import('./db');
		// No snapshot and no cleared cache: the store reads null and refreshes from /bootstrap.
		expect(await cache.readAll()).toBeNull();
		const meta = await cache.readMeta();
		expect(meta.pending).toEqual(PENDING);
		expect(meta.lastSyncAt).toBeNull();
		expect(await cache.getDraft('clienta:new', z.object({ name: z.string() }))).toEqual({
			name: 'Lucía Rojas'
		});
	});
});
