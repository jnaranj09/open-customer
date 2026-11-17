// The Copiar buttons mobile polish S3 adds (docs/designs/mobile-polish.md, section 3): the message
// of a "Para enviar" card (both kinds), the template preview and the client's phone. Cotizar and
// Respuestas are in quote.spec.ts and replies.spec.ts. The clipboard is stubbed as there. The data
// is the setup's: the 1-day "before" rule with "Recordatorio de cita", the 21-day come-back rule on
// Acrílico with "Relleno"; this spec adds only clients and appointments, which `resetData` clears.
import { expect, test, type Page } from '@playwright/test';
import es from '../../src/lib/i18n/es.json' with { type: 'json' };
import { at, closePool, insertAppointment, insertClient, resetData } from './seed';
import { COPY_REFUSALS, copied, copyFrom, copyRefused, dueItem, stubClipboard } from './steps';

const PHONE = '+50688887777';

let anaId = '';

test.beforeEach(async () => {
	await resetData();
	anaId = await insertClient({
		name: 'Ana Rojas',
		phone: PHONE,
		attrs: { 'Bebida favorita': 'Café' }
	});
});

test.afterAll(closePool);

/** Ana's appointment tomorrow, which the "before" rule makes due today; opens Para enviar. */
async function anaDueTomorrow(page: Page): Promise<void> {
	await insertAppointment({ clientId: anaId, services: ['Acrílico'], startsAt: at(1, '15:00') });
	await page.goto('/enviar');
	await expect(dueItem(page, 'Ana Rojas')).toBeVisible();
}

test.describe('Para enviar', () => {
	test('Copiar copies the message the card shows', async ({ page }) => {
		await stubClipboard(page, 'ok');
		await anaDueTomorrow(page);
		const item = dueItem(page, 'Ana Rojas');

		const text = await copyFrom(page, item);

		expect(text).toContain('Ana Rojas');
		expect(text).toContain('Café');
		await expect(item.getByText(text, { exact: true })).toBeVisible();
	});

	test('a booking prompt copies its message, not the question', async ({ page }) => {
		await stubClipboard(page, 'ok');
		// A come-back message due today (Acrílico 21 days ago) with a later Pedicure booking.
		await insertAppointment({
			clientId: anaId,
			services: ['Acrílico'],
			startsAt: at(-21, '14:30'),
			createdAt: at(-22, '09:00')
		});
		await insertAppointment({ clientId: anaId, services: ['Pedicure'], startsAt: at(7, '10:00') });
		await page.goto('/enviar');
		const item = dueItem(page, 'Ana Rojas');
		await expect(
			item.getByRole('button', { name: es.appointment_add_service, exact: true })
		).toBeVisible();

		const text = await copyFrom(page, item);

		expect(text).toContain('relleno');
		expect(text).not.toContain('Pedicure');
		await expect(item.getByText(text, { exact: true })).toBeVisible();
	});

	test('a card whose client has no phone has Copiar too', async ({ page }) => {
		await stubClipboard(page, 'ok');
		const bea = await insertClient({ name: 'Bea Mora' });
		await insertAppointment({ clientId: bea, services: ['Acrílico'], startsAt: at(1, '11:00') });
		await page.goto('/enviar');
		const item = dueItem(page, 'Bea Mora');
		await expect(item.getByRole('link', { name: es.due_add_phone })).toBeVisible();

		expect(await copyFrom(page, item)).toContain('Bea Mora');
	});

	for (const mode of COPY_REFUSALS) {
		test(`a refused copy (${mode}) says so in the toast`, async ({ page }) => {
			await stubClipboard(page, mode);
			await anaDueTomorrow(page);

			await copyRefused(page, dueItem(page, 'Ana Rojas'));
			expect(await copied(page)).toBeNull();
		});
	}
});

test.describe('template preview', () => {
	/** Opens the setup template "Recordatorio de cita"; its preview shows the sample client. */
	async function openTemplate(page: Page): Promise<void> {
		await page.goto('/ajustes/plantillas');
		await page.getByRole('link', { name: /^Recordatorio de cita/ }).click();
		await expect(
			page.getByRole('heading', { name: 'Recordatorio de cita', level: 2 })
		).toBeVisible();
	}

	const preview = (page: Page) =>
		page.getByRole('region', { name: es.preview_with }).getByRole('status');

	test('Copiar copies the rendered preview', async ({ page }) => {
		await stubClipboard(page, 'ok');
		await openTemplate(page);
		await expect(preview(page)).toContainText('te esperamos mañana.');

		const text = await copyFrom(page, page.getByRole('region', { name: es.preview_with }));

		expect(text).toContain('te esperamos mañana.');
		await expect(preview(page)).toHaveText(text);
	});

	test('an empty message has no preview and no Copiar', async ({ page }) => {
		await page.goto('/ajustes/plantillas/nueva');
		await expect(preview(page)).toHaveText(es.preview_empty);
		await expect(page.getByRole('button', { name: es.copy, exact: true })).toHaveCount(0);
	});

	for (const mode of COPY_REFUSALS) {
		test(`a refused copy (${mode}) says so in the toast`, async ({ page }) => {
			await stubClipboard(page, mode);
			await openTemplate(page);
			await expect(preview(page)).toContainText('te esperamos mañana.');

			await copyRefused(page, page.getByRole('region', { name: es.preview_with }));
			expect(await copied(page)).toBeNull();
		});
	}
});

test.describe('client phone', () => {
	test('Copiar copies the phone as stored, with its country code', async ({ page }) => {
		await stubClipboard(page, 'ok');
		await page.goto(`/clientas/${anaId}`);

		expect(await copyFrom(page, page.getByRole('main'))).toBe(PHONE);
	});

	test('a client with no phone has no Copiar', async ({ page }) => {
		const bea = await insertClient({ name: 'Bea Mora' });
		await page.goto(`/clientas/${bea}`);
		await expect(page.getByRole('button', { name: es.client_edit })).toBeVisible();
		await expect(page.getByRole('button', { name: es.copy, exact: true })).toHaveCount(0);
	});

	for (const mode of COPY_REFUSALS) {
		test(`a refused copy (${mode}) says so in the toast`, async ({ page }) => {
			await stubClipboard(page, mode);
			await page.goto(`/clientas/${anaId}`);

			await copyRefused(page, page.getByRole('main'));
			expect(await copied(page)).toBeNull();
		});
	}
});
