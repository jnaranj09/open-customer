import es from './es.json';

export type MessageKey = keyof typeof es;

const PLACEHOLDER = /\{(\w+)\}/g;

/** The Spanish text for `key`, with each `{name}` placeholder filled from `params`. A placeholder
 *  with no param throws: a half-filled message must never reach the screen. */
export function t(key: MessageKey, params?: Record<string, string | number>): string {
	return es[key].replace(PLACEHOLDER, (_match, name: string) => {
		const value = params?.[name];
		if (value === undefined) {
			throw new Error(`i18n: message ${key} needs the param ${name}`);
		}
		return String(value);
	});
}
