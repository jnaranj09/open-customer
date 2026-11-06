// Moving between screens and keeping what she typed: Back after a new form (M4), the appointment's
// back arrow (M7), a typed amount through a status change (M8), the archived list after a restore
// (M9) and the client's history after "Ver todas las citas" (L10).
import { expect, test } from '@playwright/test';
import es from '../../src/lib/i18n/es.json' with { type: 'json' };
import { at, closePool, insertAppointment, insertClient, resetData } from './seed';

let anaId = '';

test.beforeEach(async () => {
	await resetData();
	anaId = await insertClient({ name: 'Ana Rojas', phone: '+50688887777' });
});

test.afterAll(closePool);

test('Back after saving a new client does not open the empty form again', async ({ page }) => {
	await page.goto('/clientas');
	await page.getByRole('link', { name: es.clients_new }).first().click();
	await page.getByLabel(es.client_name, { exact: true }).fill('Lucía Mora');
	await page.getByRole('button', { name: es.client_save }).click();
	// On the phone, saving goes back to the list.
	await expect(page.getByRole('link', { name: /Lucía Mora/ })).toBeVisible();

	await page.goBack();

	await expect(page.getByRole('heading', { name: es.nav_clients, level: 1 })).toBeVisible();
	await expect(page.getByRole('heading', { name: es.client_new_title })).toHaveCount(0);
});

test('Back after cancelling a new appointment does not open the empty form again', async ({
	page
}) => {
	await page.goto('/');
	await page.getByRole('link', { name: es.appointment_new }).click();
	await expect(page.getByRole('heading', { name: es.appointment_new, level: 1 })).toBeVisible();
	await page.getByRole('button', { name: es.cancel, exact: true }).click();
	await expect(page.getByRole('heading', { name: es.today_title, level: 1 })).toBeVisible();

	await page.goBack();

	await expect(page.getByRole('heading', { name: es.appointment_new, level: 1 })).toHaveCount(0);
});

test("the appointment's back arrow returns to the client page she came from", async ({ page }) => {
	await insertAppointment({ clientId: anaId, services: ['Acrílico'], startsAt: at(1, '10:00') });
	await page.goto(`/clientas/${anaId}`);
	await page.getByRole('link', { name: /10:00/ }).click();
	await expect(page.getByRole('heading', { name: es.appointment_title, level: 1 })).toBeVisible();

	await page.getByRole('button', { name: es.appointment_back }).click();

	await expect(page.getByRole('heading', { name: 'Ana Rojas', level: 1 })).toBeVisible();
});

test("the appointment's back arrow goes to Hoy when the appointment was opened directly", async ({
	page
}) => {
	const appointment = await insertAppointment({
		clientId: anaId,
		services: ['Acrílico'],
		startsAt: at(1, '10:00')
	});
	await page.goto(`/citas/${appointment}`);
	await expect(page.getByRole('heading', { name: es.appointment_title, level: 1 })).toBeVisible();

	await page.getByRole('link', { name: es.appointment_back }).click();

	await expect(page.getByRole('heading', { name: es.today_title, level: 1 })).toBeVisible();
});

test('a typed amount paid stays when she changes the status', async ({ page }) => {
	const appointment = await insertAppointment({
		clientId: anaId,
		services: ['Acrílico'],
		startsAt: at(1, '10:00')
	});
	await page.goto(`/citas/${appointment}`);
	const paid = page.getByLabel(es.appointment_paid, { exact: true });
	await paid.fill('12');

	await page.getByRole('button', { name: es.appointment_confirm, exact: true }).click();

	await expect(page.getByText(es.appointment_updated, { exact: true })).toBeVisible();
	await expect(paid).toHaveValue('12');
});

test('"Ver todas las citas" for a client with none says she has no appointments', async ({
	page
}) => {
	await page.goto(`/clientas/${anaId}`);
	await expect(page.getByText(es.client_history_empty, { exact: true })).toBeVisible();

	await page.getByRole('button', { name: es.client_history_more }).click();

	await expect(page.getByRole('button', { name: es.client_history_more })).toHaveCount(0);
	await expect(page.getByText(es.client_history_empty, { exact: true })).toBeVisible();
});

test.describe('wide', () => {
	test.use({ viewport: { width: 1440, height: 900 } });

	test('the archived list drops a client once she is restored', async ({ page }) => {
		await page.goto(`/clientas/${anaId}`);
		await page.getByRole('button', { name: es.client_archive }).click();
		await page.getByRole('dialog').getByRole('button', { name: es.client_archive }).click();
		await expect(page.getByText(es.client_archived_note, { exact: true })).toBeVisible();
		await page.getByRole('button', { name: es.clients_show_archived }).click();
		const list = page.getByRole('main').getByRole('list').first();
		await expect(list.getByRole('link', { name: /Ana Rojas/ })).toBeVisible();

		await page.getByRole('button', { name: es.client_restore }).click();

		await expect(page.getByText(es.client_archived_note, { exact: true })).toBeHidden();
		await expect(list.getByRole('link', { name: /Ana Rojas/ })).toHaveCount(0);
		await expect(page.getByText(es.clients_archived_empty, { exact: true })).toBeVisible();
	});
});
