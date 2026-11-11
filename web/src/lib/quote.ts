// The text she copies into WhatsApp to quote services (design doc "The quote text"; the detail block
// of service descriptions: reminders-and-replies design, "Service description"). Pure: no Svelte,
// no Dexie, no network. The screen shows `quoteText` and Copiar copies the same string.
import { z } from 'zod';
import type { Service } from './api/schemas';
import { durationOf, LineSchema, type Line } from './booking';
import { t } from './i18n';
import { formatMoney } from './money';

/** The open quote as a draft: the picked lines. */
export const QuoteDraftSchema = z.object({ lines: z.array(LineSchema) });
export type QuoteDraft = z.infer<typeof QuoteDraftSchema>;

type Settings = { name: string; currency: string; locale: string };

/** One picked service: its name, how many copies, and its unit price (null for none). */
type Row = { name: string; count: number; unitMinor: number | null };

const MINUTES_PER_HOUR = 60;

function rowsOf(lines: Line[], services: Service[]): Row[] {
	const rows: Row[] = [];
	for (const line of lines) {
		const service = services.find((s) => s.id === line.service_id);
		if (service !== undefined) {
			rows.push({ name: service.name, count: line.count, unitMinor: service.price_cents });
		}
	}
	return rows;
}

function rowText(row: Row, currency: string, locale: string): string {
	const { name, count, unitMinor } = row;
	if (unitMinor === null) {
		const price = t('services_price_unset');
		return count === 1
			? t('quote_line', { name, price })
			: t('quote_line_unpriced_repeat', { name, count, price });
	}
	if (count === 1) {
		return t('quote_line', { name, price: formatMoney(unitMinor, currency, locale) });
	}
	return t('quote_line_repeat', {
		name,
		count,
		unit: formatMoney(unitMinor, currency, locale),
		total: formatMoney(unitMinor * count, currency, locale)
	});
}

function durationText(minutes: number): string {
	if (minutes < MINUTES_PER_HOUR) {
		return t('service_minutes', { n: minutes });
	}
	const h = Math.floor(minutes / MINUTES_PER_HOUR);
	const m = minutes % MINUTES_PER_HOUR;
	return m === 0 ? t('duration_hours', { h }) : t('duration_hours_minutes', { h, m });
}

/** The quote for `lines`, one text line per row. A line whose service is not in `services` is
 *  skipped; the caller passes only the lines it shows. */
export function quoteText(lines: Line[], services: Service[], settings: Settings): string {
	const { currency, locale } = settings;
	const name = settings.name.trim();
	const rows = rowsOf(lines, services);
	const out = [name === '' ? t('quote_heading') : t('quote_heading_named', { name })];
	for (const row of rows) {
		out.push(rowText(row, currency, locale));
	}
	const priced = rows.filter((r) => r.unitMinor !== null);
	if (priced.length > 0) {
		const totalMinor = priced.reduce((sum, r) => sum + r.count * (r.unitMinor ?? 0), 0);
		const amount = formatMoney(totalMinor, currency, locale);
		out.push(
			priced.length === rows.length
				? t('quote_total', { amount })
				: t('quote_total_partial', { amount })
		);
	}
	out.push(
		t('quote_duration', {
			duration: durationText(durationOf(lines, services))
		})
	);
	const details = detailLines(lines, services);
	if (details.length > 0) {
		out.push('', ...details);
	}
	return out.join('\n');
}

/** "Acrílico: Incluye retiro": one line per picked service that has a description, once per
 *  service, in the order of the price lines. */
function detailLines(lines: Line[], services: Service[]): string[] {
	const seen = new Set<string>();
	const out: string[] = [];
	for (const line of lines) {
		const service = services.find((s) => s.id === line.service_id);
		if (service?.description == null || seen.has(service.id)) {
			continue;
		}
		seen.add(service.id);
		out.push(t('quote_detail_line', { name: service.name, description: service.description }));
	}
	return out;
}
