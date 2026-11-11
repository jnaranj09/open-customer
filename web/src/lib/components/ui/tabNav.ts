// The pure part of `TabNav` (reminders-and-replies design, "Tabs and places"): which tab is current.
import type { ResolvedPathname } from '$app/types';
import type { MessageKey } from '../../i18n';

/** One tab: its own URL, its text, and how the current path matches it. */
export type Tab = {
	href: ResolvedPathname;
	label: MessageKey;
	/** `exact`: only `href`. `prefix`: `href` or any path under it. */
	match: 'exact' | 'prefix';
};

/** Whether `tab` is the current one at `path`. */
export function isCurrent(tab: Tab, path: string): boolean {
	return path === tab.href || (tab.match === 'prefix' && path.startsWith(`${tab.href}/`));
}
