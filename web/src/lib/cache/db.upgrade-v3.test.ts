// The Dexie upgrade from version 3 (rules gained `handover_from`, due items their label fields), in
// its own file: the cache module opens its database on import.
import { describe, expect, it } from 'vitest';
import { PENDING, seedOldCache } from './upgradeSeed';

describe('upgrading the cache from version 3', () => {
	it('drops the cached due list of the old shape and keeps the pending confirmation', async () => {
		// A version-3 due row: the item has no label fields.
		await seedOldCache(3, 'due', {
			id: 'a:1',
			position: 0,
			item: { id: 'a:1', kind: 'come_back', client_name: 'Ana Rojas' }
		});

		const cache = await import('./db');
		expect(await cache.readAll()).toBeNull();
		const meta = await cache.readMeta();
		expect(meta.pending).toEqual(PENDING);
		expect(meta.lastSyncAt).toBeNull();
	});
});
