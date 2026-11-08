import { describe, expect, it } from 'vitest';
import type { Service } from './api/schemas';
import type { Line } from './booking';
import { t } from './i18n';
import { formatMoney } from './money';
import { QuoteDraftSchema, quoteText, type QuoteDraft } from './quote';

const ACRILICO = '11111111-1111-4111-8111-111111111111';
const DISENO = '22222222-2222-4222-8222-222222222222';
const CEJAS = '33333333-3333-4333-8333-333333333333';
const GRATIS = '44444444-4444-4444-8444-444444444444';
const MISSING = '55555555-5555-4555-8555-555555555555';

const service = (id: string, name: string, duration_min: number, price_cents: number | null) =>
	({
		id,
		name,
		duration_min,
		price_cents,
		color: 'rose',
		position: 0,
		archived: false
	}) satisfies Service;

const SERVICES = [
	service(ACRILICO, 'Acrílico', 60, 1500000),
	service(DISENO, 'Diseño sencillo', 30, 300000),
	service(CEJAS, 'Cejas', 45, null),
	service(GRATIS, 'Prueba', 15, 0)
];

const SETTINGS = { name: 'Estudio Ejemplo', currency: 'CRC', locale: 'es-CR' };
const money = (minor: number, currency = SETTINGS.currency) =>
	formatMoney(minor, currency, SETTINGS.locale);
const one = (service_id: string, count = 1): Line => ({ service_id, count });
const lines = (text: string) => text.split('\n');

describe('quoteText', () => {
	it('all priced: Total line', () => {
		const text = quoteText([one(ACRILICO), one(DISENO)], SERVICES, SETTINGS);
		expect(lines(text)).toEqual([
			t('quote_heading_named', { name: SETTINGS.name }),
			t('quote_line', { name: 'Acrílico', price: money(1500000) }),
			t('quote_line', { name: 'Diseño sencillo', price: money(300000) }),
			t('quote_total', { amount: money(1800000) }),
			t('quote_duration', { duration: t('duration_hours_minutes', { h: 1, m: 30 }) })
		]);
	});

	it('some unpriced: partial total', () => {
		const text = quoteText([one(ACRILICO), one(CEJAS)], SERVICES, SETTINGS);
		expect(lines(text)).toEqual([
			t('quote_heading_named', { name: SETTINGS.name }),
			t('quote_line', { name: 'Acrílico', price: money(1500000) }),
			t('quote_line', { name: 'Cejas', price: t('services_price_unset') }),
			t('quote_total_partial', { amount: money(1500000) }),
			t('quote_duration', { duration: t('duration_hours_minutes', { h: 1, m: 45 }) })
		]);
	});

	it('none priced: no total line', () => {
		const text = quoteText([one(CEJAS)], SERVICES, SETTINGS);
		expect(lines(text)).toEqual([
			t('quote_heading_named', { name: SETTINGS.name }),
			t('quote_line', { name: 'Cejas', price: t('services_price_unset') }),
			t('quote_duration', { duration: t('service_minutes', { n: 45 }) })
		]);
	});

	it('a price of 0 is a price', () => {
		const text = quoteText([one(GRATIS)], SERVICES, SETTINGS);
		expect(lines(text)).toEqual([
			t('quote_heading_named', { name: SETTINGS.name }),
			t('quote_line', { name: 'Prueba', price: money(0) }),
			t('quote_total', { amount: money(0) }),
			t('quote_duration', { duration: t('service_minutes', { n: 15 }) })
		]);
	});

	it('repeated priced line shows ×2 and c/u', () => {
		const text = quoteText([one(DISENO, 2)], SERVICES, SETTINGS);
		expect(lines(text)[1]).toBe(
			t('quote_line_repeat', {
				name: 'Diseño sencillo',
				count: 2,
				unit: money(300000),
				total: money(600000)
			})
		);
		expect(lines(text)[1]).toContain('×2');
		expect(lines(text)[1]).toContain('c/u');
		expect(lines(text)[2]).toBe(t('quote_total', { amount: money(600000) }));
	});

	it('repeated unpriced line', () => {
		const text = quoteText([one(CEJAS, 2)], SERVICES, SETTINGS);
		expect(lines(text)).toEqual([
			t('quote_heading_named', { name: SETTINGS.name }),
			t('quote_line_unpriced_repeat', {
				name: 'Cejas',
				count: 2,
				price: t('services_price_unset')
			}),
			t('quote_duration', { duration: t('duration_hours_minutes', { h: 1, m: 30 }) })
		]);
	});

	it('repeated line ×25', () => {
		const text = quoteText([one(DISENO, 25)], SERVICES, SETTINGS);
		expect(lines(text)[1]).toBe(
			t('quote_line_repeat', {
				name: 'Diseño sencillo',
				count: 25,
				unit: money(300000),
				total: money(7500000)
			})
		);
		expect(lines(text)[3]).toBe(
			t('quote_duration', { duration: t('duration_hours_minutes', { h: 12, m: 30 }) })
		);
	});

	it('blank business name uses quote_heading', () => {
		const text = quoteText([one(ACRILICO)], SERVICES, { ...SETTINGS, name: '   ' });
		expect(lines(text)[0]).toBe(t('quote_heading'));
	});

	it.each([
		[45, t('service_minutes', { n: 45 })],
		[120, t('duration_hours', { h: 2 })],
		[150, t('duration_hours_minutes', { h: 2, m: 30 })]
	])('durations 45, 120 and 150 minutes: %i', (minutes, words) => {
		const svc = service(ACRILICO, 'Acrílico', minutes, 1000);
		const text = quoteText([one(ACRILICO)], [svc], SETTINGS);
		expect(lines(text).at(-1)).toBe(t('quote_duration', { duration: words }));
	});

	it('a line whose service is missing is skipped', () => {
		const text = quoteText([one(MISSING), one(ACRILICO)], SERVICES, SETTINGS);
		expect(lines(text)).toEqual([
			t('quote_heading_named', { name: SETTINGS.name }),
			t('quote_line', { name: 'Acrílico', price: money(1500000) }),
			t('quote_total', { amount: money(1500000) }),
			t('quote_duration', { duration: t('duration_hours', { h: 1 }) })
		]);
	});

	it('quoteText prints names literally', () => {
		const odd = service(DISENO, '{price} $& {count}', 30, 1000);
		const text = quoteText([one(DISENO)], [odd], { ...SETTINGS, name: '{name} $1' });
		expect(lines(text)[0]).toBe(t('quote_heading_named', { name: '{name} $1' }));
		expect(lines(text)[1]).toBe(`• {price} $& {count}: ${money(1000)}`);
	});

	it('quoteText uses the settings currency', () => {
		const settings = { name: 'Studio', currency: 'USD', locale: 'en-US' };
		const text = quoteText([one(ACRILICO)], SERVICES, settings);
		expect(lines(text)[1]).toBe(
			t('quote_line', { name: 'Acrílico', price: formatMoney(1500000, 'USD', 'en-US') })
		);
	});
});

describe('QuoteDraftSchema', () => {
	it('accepts a draft of lines', () => {
		const draft: QuoteDraft = { lines: [one(ACRILICO, 2)] };
		expect(QuoteDraftSchema.safeParse(draft).success).toBe(true);
	});

	it('QuoteDraftSchema rejects a bad draft', () => {
		expect(
			QuoteDraftSchema.safeParse({ lines: [{ service_id: ACRILICO, count: 0 }] }).success
		).toBe(false);
		expect(QuoteDraftSchema.safeParse({}).success).toBe(false);
	});
});
