// The pure parts of the Ajustes screens (design doc item 77): the back bar and the Negocio form.
import type { Settings } from './api/schemas';
import type { components } from './api/schema';

type SettingsPatch = components['schemas']['SettingsPatch'];

/** The latest hour of the day push can go out at (design doc "Settings": 0 to 19). */
export const LAST_PUSH_HOUR = 19;

/** True when the compact Ajustes layout shows its "Volver a Ajustes" bar above the page: on every
 *  section but the root, and not on a template's editor, which has its own back to the list. */
export function showSettingsBack(pathname: string): boolean {
	const path = pathname.replace(/\/$/, '');
	if (path === '/ajustes' || path === '') {
		return false;
	}
	return !path.startsWith('/ajustes/plantillas/');
}

export type BusinessForm = { name: string; timezone: string; color: string; pushHour: string };

/** The fields of the form that differ from the saved settings, name trimmed and zone as typed. */
export function businessPatch(saved: Settings, form: BusinessForm): SettingsPatch {
	const name = form.name.trim();
	const timezone = form.timezone.trim();
	const hour = Number(form.pushHour);
	return {
		...(name === saved.name ? {} : { name }),
		...(timezone === saved.timezone ? {} : { timezone }),
		...(form.color.toLowerCase() === saved.accent_color ? {} : { accent_color: form.color }),
		...(hour === saved.push_hour ? {} : { push_hour: hour })
	};
}
