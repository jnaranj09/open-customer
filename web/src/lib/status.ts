// The pure parts of Ajustes → Estado del sistema (design doc "Status screen"): the report as the
// lines the screen lists, with the red ones marked.
import type { Status } from './api/schemas';
import { t, type MessageKey } from './i18n';
import { formatDayWithYear, formatTime, todayIn } from './time';

export type StatusLine = { label: MessageKey; value: string; red: boolean };

/** "hace 45 s", "hace 3 min", "hace 2 h", "hace 2 d". */
export function ageText(seconds: number): string {
	if (seconds < 60) {
		return t('age_seconds', { n: seconds });
	}
	if (seconds < 3600) {
		return t('age_minutes', { n: Math.floor(seconds / 60) });
	}
	if (seconds < 86400) {
		return t('age_hours', { n: Math.floor(seconds / 3600) });
	}
	return t('age_days', { n: Math.floor(seconds / 86400) });
}

const NUMBER = new Intl.NumberFormat('es', { maximumFractionDigits: 1 });

/** "12,3 GB", "450 MB". */
export function bytesText(bytes: number): string {
	if (bytes >= 1e9) {
		return `${NUMBER.format(bytes / 1e9)} GB`;
	}
	return `${NUMBER.format(bytes / 1e6)} MB`;
}

/** The lines of the status screen, in order. `now` gives the business day, so a date from another
 *  year shows its year. */
export function statusLines(status: Status, zone: string, now: Date): StatusLine[] {
	const today = todayIn(zone, now);
	const dayTime = (iso: string): string =>
		`${formatDayWithYear(todayIn(zone, new Date(iso)), today)} ${formatTime(iso, zone)}`;
	const { backup, push } = status;
	const pushValue = push.red
		? t('status_push_red')
		: t(push.subscriptions === 1 ? 'status_push_one' : 'status_push_other', {
				n: push.subscriptions
			});
	const last = push.last_push;
	return [
		{ label: 'status_version', value: status.version, red: false },
		{ label: 'status_migration', value: String(status.migration_version), red: false },
		{
			label: 'status_timer',
			value:
				status.heartbeat_age_s === null ? t('status_timer_none') : ageText(status.heartbeat_age_s),
			red: false
		},
		{
			label: 'status_backup',
			value: backup.red
				? t('backup_red')
				: backup.last_copy_at === null
					? t('status_backup_none')
					: dayTime(backup.last_copy_at),
			red: backup.red
		},
		{
			label: 'status_space',
			value:
				status.media_free_bytes === null
					? t('status_space_unknown')
					: bytesText(status.media_free_bytes),
			red: false
		},
		{ label: 'status_push', value: pushValue, red: push.red },
		{
			label: 'status_last_push',
			value:
				last === null
					? t('status_last_push_none')
					: t('status_last_push_result', {
							day: formatDayWithYear(last.date, today),
							delivered: last.delivered,
							attempted: last.attempted
						}),
			red: false
		}
	];
}
