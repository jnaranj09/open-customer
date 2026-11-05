// The pure parts of Ajustes → Servicios (design doc item 77): the form to a request, the patch of
// what changed, and the row's second line.
import type { Service } from './api/schemas';
import type { components } from './api/schema';
import { t, type MessageKey } from './i18n';
import { parseAmount } from './money';

type Schemas = components['schemas'];

/** What the form holds, as typed. */
export type ServiceForm = { name: string; duration: string; price: string; color: string };

export type ServiceField = 'name' | 'duration' | 'price';

/** The request body of a create; `price_cents` is null for no price. */
export type ServiceBody = Required<
	Pick<Schemas['ServiceInput'], 'name' | 'duration_min' | 'color'>
> & {
	price_cents: number | null;
};

const MAX_MINUTES = 1440;

/** The request for the form, or the message key of each field that is not valid. `exponent` is the
 *  currency's decimals (`exponentOf`). */
export function serviceRequest(
	form: ServiceForm,
	exponent: number
):
	| { ok: true; body: ServiceBody }
	| { ok: false; errors: Partial<Record<ServiceField, MessageKey>> } {
	const errors: Partial<Record<ServiceField, MessageKey>> = {};
	const name = form.name.trim();
	if (name === '') {
		errors.name = 'service_name_required';
	}
	const minutes = /^\d+$/.test(form.duration.trim()) ? Number(form.duration.trim()) : 0;
	if (minutes < 1 || minutes > MAX_MINUTES) {
		errors.duration = 'service_duration_invalid';
	}
	let price: number | null = null;
	if (form.price.trim() !== '') {
		price = parseAmount(form.price, exponent);
		if (price === null) {
			errors.price = 'service_price_invalid';
		}
	}
	if (Object.keys(errors).length > 0) {
		return { ok: false, errors };
	}
	return { ok: true, body: { name, duration_min: minutes, price_cents: price, color: form.color } };
}

/** The fields of `body` that differ from the saved service. */
export function servicePatch(saved: Service, body: ServiceBody): Schemas['ServicePatch'] {
	return {
		...(body.name === saved.name ? {} : { name: body.name }),
		...(body.duration_min === saved.duration_min ? {} : { duration_min: body.duration_min }),
		...(body.price_cents === saved.price_cents ? {} : { price_cents: body.price_cents }),
		...(body.color === saved.color ? {} : { color: body.color })
	};
}

/** "90 min · ₡15 000": the second line of a service row. `money` writes an amount in minor units. */
export function serviceSubtitle(service: Service, money: (minor: number) => string): string {
	const minutes = t('service_minutes', { n: service.duration_min });
	return service.price_cents === null ? minutes : `${minutes} · ${money(service.price_cents)}`;
}
