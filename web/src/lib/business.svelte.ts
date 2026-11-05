// The business time zone, currency and locale from the cached settings, with the browser's zone
// before the first refresh.
import { cache } from './cache/store.svelte';

export const business = {
	get zone(): string {
		return cache.snapshot?.settings.timezone ?? Intl.DateTimeFormat().resolvedOptions().timeZone;
	},
	get currency(): string {
		return cache.snapshot?.settings.currency ?? '';
	},
	get locale(): string {
		return cache.snapshot?.settings.locale ?? 'es';
	}
};
