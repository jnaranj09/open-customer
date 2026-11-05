// The pure parts of the setup wizard (design doc item 73): the steps, the choice lists built from
// `Intl`, and the setup token. The page owns the state and the calls.

export type WizardStep =
	| 'business'
	| 'timezone'
	| 'currency'
	| 'country'
	| 'color'
	| 'username'
	| 'password'
	| 'icon'
	| 'install';

/** The wizard's order. `POST /setup` runs when she leaves `password`; `icon` and `install` come after it. */
export const STEPS: readonly WizardStep[] = [
	'business',
	'timezone',
	'currency',
	'country',
	'color',
	'username',
	'password',
	'icon',
	'install'
];

export type Option = { id: string; label: string };

/** The slice of `Intl` this file uses, so a test passes a fake. The real `Intl` fits it. */
export type IntlLike = {
	/** Missing before iOS 15.4. */
	supportedValuesOf?: (key: 'timeZone' | 'currency') => string[];
	DateTimeFormat: new (
		locale: string,
		options: { timeZone: string; timeZoneName: 'longGeneric' }
	) => { formatToParts: (date: Date) => { type: string; value: string }[] };
	DisplayNames: new (
		locales: string[],
		options: { type: 'currency' | 'region' }
	) => { of: (code: string) => string | undefined };
};

const LOCALE = 'es';

/** The token after the `#` of the setup link, or null when there is none. */
export function tokenFromHash(hash: string): string | null {
	const token = hash.startsWith('#') ? hash.slice(1) : hash;
	return token === '' ? null : token;
}

function zoneLabel(intl: IntlLike, id: string): string {
	try {
		const name = new intl.DateTimeFormat(LOCALE, { timeZone: id, timeZoneName: 'longGeneric' })
			.formatToParts(new Date(0))
			.find((p) => p.type === 'timeZoneName')?.value;
		return name === undefined ? id : `${name} (${id})`;
	} catch {
		// A zone the browser lists but cannot format (a RangeError): its id is label enough.
		return id;
	}
}

/** Every time zone by IANA id, the browser's first; null when the browser cannot list them (the
 *  field is then a text box). */
export function timeZoneOptions(intl: IntlLike, browserZone: string): Option[] | null {
	if (intl.supportedValuesOf === undefined) {
		return null;
	}
	const ids = [browserZone, ...intl.supportedValuesOf('timeZone').filter((z) => z !== browserZone)];
	return ids.map((id) => ({ id, label: zoneLabel(intl, id) }));
}

/** Every ISO 4217 code with its Spanish name; null when the browser cannot list them. */
export function currencyOptions(intl: IntlLike): Option[] | null {
	if (intl.supportedValuesOf === undefined) {
		return null;
	}
	const names = new intl.DisplayNames([LOCALE], { type: 'currency' });
	return intl.supportedValuesOf('currency').map((id) => {
		const name = names.of(id);
		return { id, label: name === undefined ? id : `${id} · ${name}` };
	});
}

/** The server's country codes with their Spanish names, sorted by name. */
export function countryOptions(codes: readonly string[], intl: IntlLike): Option[] {
	const names = new intl.DisplayNames([LOCALE], { type: 'region' });
	return codes
		.map((id) => ({ id, label: names.of(id) ?? id }))
		.sort((a, b) => a.label.localeCompare(b.label, LOCALE));
}

const FIELD_STEPS: Record<string, WizardStep> = {
	'settings.name': 'business',
	'settings.timezone': 'timezone',
	'settings.currency': 'currency',
	'settings.country_code': 'country',
	'settings.accent_color': 'color',
	username: 'username',
	password: 'password'
};

/** The step that holds the field a `POST /setup` validation error names, or null. */
export function stepForField(field: string): WizardStep | null {
	return FIELD_STEPS[field] ?? null;
}
