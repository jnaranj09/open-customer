import type { MessageKey } from '../i18n';
import type { IconName } from './icons/Icon.svelte';

export type NavItem = {
	href: '/' | '/enviar' | '/clientas' | '/ajustes';
	icon: IconName;
	/** The sidebar label. */
	label: MessageKey;
	/** The tab bar label, shorter where the label would not fit. */
	tabLabel: MessageKey;
};

/** The four places, in the order the sidebar and the tab bar show them. */
export const NAV_ITEMS: readonly NavItem[] = [
	{ href: '/', icon: 'calendar-days', label: 'nav_today', tabLabel: 'nav_today' },
	{ href: '/enviar', icon: 'send', label: 'nav_to_send', tabLabel: 'tab_send' },
	{ href: '/clientas', icon: 'users', label: 'nav_clients', tabLabel: 'nav_clients' },
	{ href: '/ajustes', icon: 'settings', label: 'nav_settings', tabLabel: 'nav_settings' }
];
