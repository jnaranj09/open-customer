// The browser tab and app-switcher title of every screen (bug hunt L1), set once by the root
// layout. Keyed by SvelteKit route id: the type makes a new route without a title a type error.
import type { PageRouteId } from '$app/types';
import { t, type MessageKey } from './i18n';

const TITLES: Record<PageRouteId, MessageKey> = {
	'/': 'today_title',
	'/ajustes': 'settings_title',
	'/ajustes/atributos': 'attributes_title',
	'/ajustes/estado': 'status_title',
	'/ajustes/exportar': 'export_title',
	'/ajustes/negocio': 'business_title',
	'/ajustes/(reglas)/plantillas': 'templates_title',
	'/ajustes/(reglas)/plantillas/[id]': 'page_title_template',
	'/ajustes/(reglas)/plantillas/nueva': 'templates_new_title',
	'/ajustes/(reglas)/recordatorios': 'rules_title',
	'/cambiar-clave': 'password_title',
	'/citas/[id]': 'appointment_title',
	'/citas/nueva': 'appointment_new',
	'/clientas': 'nav_clients',
	'/clientas/[id]': 'page_title_client',
	'/clientas/nueva': 'client_new_title',
	'/enviar': 'due_today_heading',
	'/enviar/respuestas': 'tab_replies',
	'/login': 'page_title_login',
	'/servicios': 'quote_title',
	'/servicios/catalogo': 'services_title',
	'/setup': 'setup_title'
};

/** The Spanish `<title>` of the screen at `routeId` (`page.route.id`); the app's name for a path
 *  with no route (the 404 page). */
export function pageTitle(routeId: string | null): string {
	const byId: Partial<Record<string, MessageKey>> = TITLES;
	const key = routeId === null ? undefined : byId[routeId];
	return t(key ?? 'page_title_app');
}
