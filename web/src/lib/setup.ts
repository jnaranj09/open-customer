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

/** The wizard's steps after `POST /setup`: the account exists, so a reload must come back to them
 *  instead of leaving the wizard (the guard sends a signed-in user away from `/setup`). */
export type FinishingStep = 'icon' | 'install';

/** The tab's session storage key that holds the finishing step. */
const FINISHING_KEY = 'open-customer.setup-finishing';

/** The slice of `Storage` the finishing step uses; a test passes a plain object. */
export type StepStore = Pick<Storage, 'getItem' | 'setItem' | 'removeItem'>;

/** The tab's session storage, or null where the browser refuses it (private mode, settings). */
export function sessionStore(): StepStore | null {
	try {
		return window.sessionStorage;
	} catch {
		console.warn('code=session_storage_unavailable');
		return null;
	}
}

/** The finishing step stored in `store`, or null when the wizard is not between its account and
 *  its end. */
export function finishingStep(store: StepStore | null): FinishingStep | null {
	try {
		const step = store?.getItem(FINISHING_KEY) ?? null;
		return step === 'icon' || step === 'install' ? step : null;
	} catch {
		console.warn('code=session_storage_read_failed');
		return null;
	}
}

/** Stores the finishing step, or clears it with null (she entered the app). A failed write only
 *  loses the reload case: the wizard goes on. */
export function rememberFinishing(store: StepStore | null, step: FinishingStep | null): void {
	try {
		if (step === null) {
			store?.removeItem(FINISHING_KEY);
		} else {
			store?.setItem(FINISHING_KEY, step);
		}
	} catch {
		console.warn('code=session_storage_write_failed');
	}
}
