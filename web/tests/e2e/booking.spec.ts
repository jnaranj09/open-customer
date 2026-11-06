// CUJ 3 (design doc item 79): booking an appointment, the overlap warning, and the photo step. It
// builds on the services and the "Forma" attribute that `setup.spec.ts` creates.
import { readFileSync } from 'node:fs';
import { expect, test, type Page } from '@playwright/test';
import es from '../../src/lib/i18n/es.json' with { type: 'json' };
import { at, businessDate, closePool, insertAppointment, insertClient, resetData } from './seed';

const PHOTO = 'tests/e2e/fixtures/photo-gps.jpg';
/** The Exif APP1 identifier: the fixture carries it, the stored photo must not. */
const EXIF = Buffer.from('Exif\0\0', 'binary');

/** Days from today to the next Saturday, at least 1. */
function saturdayOffset(): number {
	const weekday = new Date(`${businessDate(0)}T00:00:00Z`).getUTCDay();
	return (6 - weekday + 7) % 7 || 7;
}
const SATURDAY = saturdayOffset();

let anaId = '';

test.beforeEach(async () => {
	await resetData();
	anaId = await insertClient({ name: 'Ana Rojas', phone: '+50688887777' });
	await insertClient({ name: 'Ána López', phone: '+50688886666' });
});

test.afterAll(closePool);

/** Hoy → "+" → Ana Rojas, Acrílico and Pedicure, next Saturday 15:00, "Forma: Almendra" → save. */
async function bookAnaOnSaturday(page: Page): Promise<void> {
	await page.goto('/');
	await page.getByRole('link', { name: es.appointment_new }).click();
	await page.getByLabel(es.appointment_client, { exact: true }).fill('ana');
	await expect(page.getByRole('button', { name: 'Ána López', exact: true })).toBeVisible();
	await page.getByRole('button', { name: 'Ana Rojas', exact: true }).click();
	await page.getByRole('button', { name: 'Acrílico', exact: true }).click();
	await page.getByRole('button', { name: 'Pedicure', exact: true }).click();
	await page.getByLabel(es.appointment_day, { exact: true }).fill(businessDate(SATURDAY));
	await page.getByLabel(es.appointment_time, { exact: true }).fill('15:00');
	await page.getByLabel('Forma', { exact: true }).selectOption({ label: 'Almendra' });
	await page.getByRole('button', { name: es.appointment_save }).click();
}

/** Hoy, moved to the next Saturday. */
async function openSaturday(page: Page): Promise<void> {
	await expect(page.getByRole('heading', { name: es.today_title, level: 1 })).toBeVisible();
	await page.getByLabel(es.agenda_pick_date).fill(businessDate(SATURDAY));
}

test('books two services with the summed end time', async ({ page }) => {
	await bookAnaOnSaturday(page);
	await openSaturday(page);
	await page.getByRole('link', { name: /Ana Rojas/ }).click();

	await expect(page.getByRole('heading', { name: 'Ana Rojas', level: 2 })).toBeVisible();
	await expect(page.getByText('15:00 – 17:30', { exact: true })).toBeVisible();
	await expect(
		page.getByText(es.appointment_total.replace('{amount}', '70,00 US$'), { exact: true })
	).toBeVisible();
	await expect(page.getByText('Almendra', { exact: true })).toBeVisible();
});

test('warns on an overlap and books with Agendar igual', async ({ page }) => {
	const marta = await insertClient({ name: 'Marta Solano' });
	await insertAppointment({
		clientId: marta,
		services: ['Relleno'],
		startsAt: at(SATURDAY, '15:30')
	});

	await bookAnaOnSaturday(page);
	const sheet = page.getByRole('dialog', { name: es.overlap_title });
	await expect(sheet.getByText('Marta Solano')).toBeVisible();
	await sheet.getByRole('button', { name: es.overlap_confirm }).click();

	await openSaturday(page);
	await expect(page.getByRole('link', { name: /Ana Rojas/ })).toBeVisible();
	await expect(page.getByRole('link', { name: /Marta Solano/ })).toBeVisible();
});

test('the status chip reads "Estado: Agendada" from its own text, with no aria-label', async ({
	page
}) => {
	const appointment = await insertAppointment({
		clientId: anaId,
		services: ['Acrílico'],
		startsAt: at(1, '10:00')
	});
	await page.goto(`/citas/${appointment}`);

	const chip = page.getByText(`${es.appointment_status_label}: ${es.status_scheduled}`, {
		exact: true
	});
	await expect(chip).toBeVisible();
	await expect(chip).not.toHaveAttribute('aria-label');
});

test('the stored photo has no Exif data', async ({ page }) => {
	// The fixture does carry Exif with GPS, so a clean stored file means the app removed it.
	expect(readFileSync(PHOTO).includes(EXIF)).toBe(true);
	const appointment = await insertAppointment({
		clientId: anaId,
		services: ['Acrílico'],
		startsAt: at(-1, '15:00')
	});

	await page.goto(`/citas/${appointment}`);
	await expect(page.getByText(es.photos_empty, { exact: true })).toBeVisible();
	// "Agregar foto" is an app button that opens the chooser (the browser's file box is hidden).
	const chooser = page.waitForEvent('filechooser');
	await page.getByRole('button', { name: es.photo_add }).click();
	await (await chooser).setFiles(PHOTO);
	const photo = page.getByRole('img', { name: es.photo_alt });
	await expect(photo).toBeVisible();

	// The grid shows the thumbnail (`/api/v1/media/{id}/thumb`); the stored photo is `/media/{id}`.
	const thumb = await photo.getAttribute('src');
	expect(thumb).toMatch(/^\/api\/v1\/media\/[0-9a-f-]+\/thumb$/);
	const stored = await page.request.get(String(thumb).replace(/\/thumb$/, ''));
	expect(stored.status()).toBe(200);
	expect((await stored.body()).includes(EXIF)).toBe(false);

	await page.goto(`/clientas/${anaId}`);
	await expect(page.getByRole('img', { name: es.photo_alt })).toBeVisible();
});
