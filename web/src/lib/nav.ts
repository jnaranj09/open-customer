import { t } from './i18n';
import { formatTime } from './time';

/** True when the nav item for `href` is the open page: `/` only on the home page, any other item on
 *  its own path and the pages below it. */
export function isActivePath(href: string, path: string): boolean {
	return href === '/' ? path === '/' : path === href || path.startsWith(`${href}/`);
}

/** The sidebar's sync line: when the data is from, or that the system is off. */
export function syncLine(lastSyncAt: string | null, zone: string, offline: boolean): string {
	if (offline) {
		return t('error_offline');
	}
	return lastSyncAt === null
		? t('sidebar_no_data')
		: t('sidebar_synced', { time: formatTime(lastSyncAt, zone) });
}
