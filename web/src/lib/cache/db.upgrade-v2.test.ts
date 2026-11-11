// The Dexie upgrade from version 2 (services gained `description`), in its own file: the cache
// module opens its database on import, so the version-2 database must exist first.
import { describe, expect, it } from 'vitest';
import { z } from 'zod';
import { PENDING, seedOldCache } from './upgradeSeed';

describe('upgrading the cache from version 2', () => {
	it('drops the cached services that lack a description, and keeps drafts and the pending confirmation', async () => {
		// A version-2 service row: no description field.
		await seedOldCache(2, 'services', {
			id: '11111111-1111-4111-8111-111111111111',
			name: 'Acrílico',
			duration_min: 90,
			price_cents: null,
			color: '#aa3366',
			position: 0,
			archived: false
		});

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
