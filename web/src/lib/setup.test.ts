import { describe, expect, it, vi } from 'vitest';
import {
	countryOptions,
	currencyOptions,
	finishingStep,
	rememberFinishing,
	storeOnce,
	stepForField,
	STEPS,
	timeZoneOptions,
	tokenFromHash,
	type IntlLike,
	type StepStore
} from './setup';

class FakeDateTimeFormat {
	constructor(
		_locale: string,
		private readonly opts: { timeZone: string }
	) {}
	formatToParts(): { type: string; value: string }[] {
		return [
			{ type: 'month', value: '1' },
			{ type: 'timeZoneName', value: `Hora de ${this.opts.timeZone}` }
		];
	}
}

class FakeDisplayNames {
	constructor(
		_locales: string[],
		private readonly opts: { type: 'currency' | 'region' }
	) {}
	of(code: string): string | undefined {
		return code === 'XXX' ? undefined : `${this.opts.type} ${code}`;
	}
}

const intl = (supported?: IntlLike['supportedValuesOf']): IntlLike => ({
	...(supported === undefined ? {} : { supportedValuesOf: supported }),
	DateTimeFormat: FakeDateTimeFormat,
	DisplayNames: FakeDisplayNames
});

const zones = ['America/Bogota', 'America/Costa_Rica', 'Europe/Madrid'];

describe('tokenFromHash', () => {
	it('reads the token after the #', () => {
		expect(tokenFromHash('#abc_-123')).toBe('abc_-123');
	});

	it('rejects an empty or missing token', () => {
		expect([tokenFromHash(''), tokenFromHash('#')]).toEqual([null, null]);
	});
});

describe('timeZoneOptions', () => {
	it('puts the browser zone first and labels each with its name', () => {
		const got = timeZoneOptions(
			intl(() => zones),
			'America/Costa_Rica'
		);
		expect(got?.map((o) => o.id)).toEqual([
			'America/Costa_Rica',
			'America/Bogota',
			'Europe/Madrid'
		]);
		expect(got?.[0]?.label).toBe('Hora de America/Costa_Rica (America/Costa_Rica)');
	});

	it('adds a browser zone the list does not hold, once, at the top', () => {
		const got = timeZoneOptions(
			intl(() => zones),
			'UTC'
		);
		expect(got?.map((o) => o.id)).toEqual(['UTC', ...zones]);
	});

	it('is null without supportedValuesOf', () => {
		expect(timeZoneOptions(intl(), 'UTC')).toBeNull();
	});
});

describe('currencyOptions', () => {
	it('labels each code with its Spanish name', () => {
		const got = currencyOptions(intl(() => ['CRC', 'USD']));
		expect(got).toEqual([
			{ id: 'CRC', label: 'CRC · currency CRC' },
			{ id: 'USD', label: 'USD · currency USD' }
		]);
	});

	it('is null without supportedValuesOf', () => {
		expect(currencyOptions(intl())).toBeNull();
	});

	it('falls back to the bare code when there is no name', () => {
		expect(currencyOptions(intl(() => ['XXX']))).toEqual([{ id: 'XXX', label: 'XXX' }]);
	});
});

describe('countryOptions', () => {
	it('sorts by Spanish name', () => {
		const got = countryOptions(['ES', 'CR'], intl());
		expect(got.map((o) => o.id)).toEqual(['CR', 'ES']);
	});
});

describe('stepForField', () => {
	it('maps each server field to the step that holds it', () => {
		expect(
			['settings.name', 'settings.timezone', 'settings.currency', 'settings.country_code'].map(
				stepForField
			)
		).toEqual(['business', 'timezone', 'currency', 'country']);
		expect(['settings.accent_color', 'username', 'password'].map(stepForField)).toEqual([
			'color',
			'username',
			'password'
		]);
	});

	it('is null for a field no step holds', () => {
		expect(stepForField('settings.push_hour')).toBeNull();
	});
});

describe('STEPS', () => {
	it('runs from the business name to the install guide', () => {
		expect([STEPS[0], STEPS.at(-1)]).toEqual(['business', 'install']);
	});
});

/** A session storage stand-in: a map behind the three methods. */
function memoryStore(): StepStore {
	const items = new Map<string, string>();
	return {
		getItem: (k) => items.get(k) ?? null,
		setItem: (k, v) => {
			items.set(k, v);
		},
		removeItem: (k) => {
			items.delete(k);
		}
	};
}

describe('finishing step', () => {
	it('is null before setup made the account', () => {
		expect(finishingStep(memoryStore())).toBeNull();
	});

	it('reads back the step the wizard stored', () => {
		const store = memoryStore();
		rememberFinishing(store, 'install');
		expect(finishingStep(store)).toBe('install');
	});

	it('is null again once she entered the app', () => {
		const store = memoryStore();
		rememberFinishing(store, 'icon');
		rememberFinishing(store, null);
		expect(finishingStep(store)).toBeNull();
	});

	it('is null without storage', () => {
		expect(finishingStep(null)).toBeNull();
	});

	it('is null when the stored value is not a finishing step', () => {
		const store = memoryStore();
		store.setItem('open-customer.setup-finishing', 'business');
		expect(finishingStep(store)).toBeNull();
	});

	it('is null when the storage throws on read', () => {
		const store = {
			...memoryStore(),
			getItem: (): string | null => {
				throw new Error('denied');
			}
		};
		expect(finishingStep(store)).toBeNull();
	});
});

describe('storeOnce', () => {
	it('tries a refused storage once and logs it once', () => {
		const warn = vi.spyOn(console, 'warn').mockImplementation(() => undefined);
		const open = vi.fn((): StepStore => {
			throw new Error('denied');
		});
		const store = storeOnce(open);
		expect([store(), store()]).toEqual([null, null]);
		expect([open.mock.calls.length, warn.mock.calls.length]).toEqual([1, 1]);
		warn.mockRestore();
	});

	it('keeps the storage it opened', () => {
		const memory = memoryStore();
		const open = vi.fn(() => memory);
		const store = storeOnce(open);
		expect([store(), store(), open.mock.calls.length]).toEqual([memory, memory, 1]);
	});
});
