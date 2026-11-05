import { describe, expect, it } from 'vitest';
import type { Service } from './api/schemas';
import { servicePatch, serviceRequest, serviceSubtitle } from './services';

const ID = '3f2b8c1e-5d4a-4e6b-9c7d-1a2b3c4d5e6f';
const PINK = ['#', 'c76b85'].join('');

const form = { name: ' Acrílico ', duration: '90', price: '15000', color: PINK };

function saved(over: Partial<Service> = {}): Service {
	return {
		id: ID,
		name: 'Acrílico',
		duration_min: 90,
		price_cents: 1500000,
		color: PINK,
		position: 0,
		archived: false,
		...over
	};
}

describe('serviceRequest', () => {
	it('turns the form into the request, with the price in minor units', () => {
		expect(serviceRequest(form, 2)).toEqual({
			ok: true,
			body: { name: 'Acrílico', duration_min: 90, price_cents: 1500000, color: PINK }
		});
	});

	it('reads a currency without decimals', () => {
		expect(serviceRequest(form, 0)).toMatchObject({ ok: true, body: { price_cents: 15000 } });
	});

	it('sends no price when the field is empty', () => {
		expect(serviceRequest({ ...form, price: ' ' }, 2)).toEqual({
			ok: true,
			body: { name: 'Acrílico', duration_min: 90, price_cents: null, color: PINK }
		});
	});

	it('asks for a name', () => {
		expect(serviceRequest({ ...form, name: '  ' }, 2)).toEqual({
			ok: false,
			errors: { name: 'service_name_required' }
		});
	});

	it('rejects a duration that is not whole minutes from 1 to 1440', () => {
		const bad = ['0', '1441', '1.5', 'x', ''].map((duration) =>
			serviceRequest({ ...form, duration }, 2)
		);
		expect(bad).toEqual(
			Array.from({ length: 5 }, () => ({
				ok: false,
				errors: { duration: 'service_duration_invalid' }
			}))
		);
	});

	it('rejects a price that is not an amount', () => {
		expect(serviceRequest({ ...form, price: '12,345' }, 2)).toEqual({
			ok: false,
			errors: { price: 'service_price_invalid' }
		});
	});
});

describe('servicePatch', () => {
	it('holds only the fields that changed', () => {
		const body = { name: 'Acrílico', duration_min: 60, price_cents: 1500000, color: PINK };
		expect(servicePatch(saved(), body)).toEqual({ duration_min: 60 });
	});

	it('clears the price with null', () => {
		const body = { name: 'Acrílico', duration_min: 90, price_cents: null, color: PINK };
		expect(servicePatch(saved(), body)).toEqual({ price_cents: null });
	});

	it('is empty when nothing changed', () => {
		const body = { name: 'Acrílico', duration_min: 90, price_cents: 1500000, color: PINK };
		expect(servicePatch(saved(), body)).toEqual({});
	});
});

describe('serviceSubtitle', () => {
	it('shows the minutes and the price', () => {
		expect(serviceSubtitle(saved({ price_cents: 1500 }), (minor) => `$${String(minor)}`)).toBe(
			'90 min · $1500'
		);
	});

	it('shows only the minutes without a price', () => {
		expect(serviceSubtitle(saved({ price_cents: null }), (minor) => String(minor))).toBe('90 min');
	});
});
