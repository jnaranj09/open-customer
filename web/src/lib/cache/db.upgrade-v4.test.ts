// The Dexie upgrade from version 4 (the new `quick_replies` table), in its own file: the cache
// module opens its database on import.
import { describe, expect, it } from 'vitest';
import { PENDING, seedOldCache } from './upgradeSeed';

describe('upgrading the cache from version 4', () => {
	it('drops the copied data so the next start fills the replies, and keeps the pending confirmation', async () => {
		await seedOldCache(4, 'templates', { id: 'a', name: 'Aviso', body: 'Hola', archived: false });

		const cache = await import('./db');
		expect(await cache.readAll()).toBeNull();
		const meta = await cache.readMeta();
		expect(meta.pending).toEqual(PENDING);
		expect(meta.lastSyncAt).toBeNull();
	});
});
