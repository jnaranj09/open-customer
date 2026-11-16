// Mobile polish S2 (docs/designs/mobile-polish.md, section 2): with no active attribute of a scope,
// the client form, the appointment form and the client detail say so, and the detail links to
// Ajustes › Atributos. With an active one the field shows and the hint does not. Builds on the
// "Bebida favorita" (client) and "Forma" (appointment) attributes that `setup.spec.ts` creates;
// a test that archives them brings them back in `afterEach`.
import { expect, test, type Page } from '@playwright/test';
import es from '../../src/lib/i18n/es.json' with { type: 'json' };
import { archiveAttributes, closePool, insertClient, resetData, restoreAttributes } from './seed';

// The bootstrap a test rewrites must come from the page, not from a service worker.
test.use({ serviceWorkers: 'block' });

let anaId = '';
let archived: string[] = [];

test.beforeEach(async () => {
	await resetData();
	anaId = await insertClient({
		name: 'Ana Rojas',
		phone: '+50688887777',
		attrs: { 'Bebida favorita': 'Café' }
	});
});

test.afterEach(async () => {
	await restoreAttributes(archived);
	archived = [];
});

test.afterAll(closePool);

/** Answers the bootstrap with no attributes at all, as in an app where she never made one. */
async function noAttributes(page: Page): Promise<void> {
	await page.route(
		(url) => url.pathname === '/api/v1/bootstrap',
		async (route) => {
			const real = await route.fetch();
			const body = (await real.json()) as Record<string, unknown>;
			await route.fulfill({ response: real, json: { ...body, attributes: [] } });
		}
	);
}

async function openClientForm(page: Page): Promise<void> {
	await page.goto('/clientas/nueva');
	await expect(page.getByLabel(es.client_name, { exact: true })).toBeVisible();
}

async function openAppointmentForm(page: Page): Promise<void> {
	await page.goto('/citas/nueva');
	await expect(page.getByRole('button', { name: 'Acrílico', exact: true })).toBeVisible();
}

async function openAna(page: Page): Promise<void> {
	await page.goto(`/clientas/${anaId}`);
	await expect(page.getByRole('heading', { name: 'Ana Rojas', level: 1 })).toBeVisible();
}

test.describe('with no attributes', () => {
	test.beforeEach(async ({ page }) => {
		await noAttributes(page);
	});

	test('the client form shows the client hint', async ({ page }) => {
		await openClientForm(page);

		await expect(page.getByText(es.attributes_none_client, { exact: true })).toBeVisible();
	});

	test('the appointment form shows the appointment hint', async ({ page }) => {
		await openAppointmentForm(page);

		await expect(page.getByText(es.attributes_none_appointment, { exact: true })).toBeVisible();
	});

	test('the client detail shows the hint, and its link opens Atributos', async ({ page }) => {
		await openAna(page);
		await expect(page.getByText(es.attributes_none_client, { exact: true })).toBeVisible();

		await page.getByRole('link', { name: es.attributes_none_link, exact: true }).click();

		await expect(page).toHaveURL(/\/ajustes\/atributos$/);
		await expect(page.getByRole('heading', { name: es.attributes_title, level: 1 })).toBeVisible();
	});
});

test.describe('with one active attribute of each scope', () => {
	test('the client form shows the field and no hint', async ({ page }) => {
		await openClientForm(page);

		await expect(page.getByLabel('Bebida favorita', { exact: true })).toBeVisible();
		await expect(page.getByText(es.attributes_none_client, { exact: true })).toHaveCount(0);
	});

	test('the appointment form shows the field and no hint', async ({ page }) => {
		await openAppointmentForm(page);

		await expect(page.getByLabel('Forma', { exact: true })).toBeVisible();
		await expect(page.getByText(es.attributes_none_appointment, { exact: true })).toHaveCount(0);
	});

	test('the client detail shows the fact and no hint', async ({ page }) => {
		await openAna(page);

		await expect(page.getByText('Café', { exact: true })).toBeVisible();
		await expect(page.getByText(es.attributes_none_client, { exact: true })).toHaveCount(0);
		await expect(page.getByRole('link', { name: es.attributes_none_link })).toHaveCount(0);
	});
});

test.describe('with only archived attributes', () => {
	test.beforeEach(async () => {
		archived = await archiveAttributes();
	});

	test('the client form shows the client hint and no field', async ({ page }) => {
		await openClientForm(page);

		await expect(page.getByText(es.attributes_none_client, { exact: true })).toBeVisible();
		await expect(page.getByLabel('Bebida favorita', { exact: true })).toHaveCount(0);
	});

	test('the appointment form shows the appointment hint and no field', async ({ page }) => {
		await openAppointmentForm(page);

		await expect(page.getByText(es.attributes_none_appointment, { exact: true })).toBeVisible();
		await expect(page.getByLabel('Forma', { exact: true })).toHaveCount(0);
	});

	test('the client detail shows the hint and still the archived fact', async ({ page }) => {
		await openAna(page);

		await expect(page.getByText(es.attributes_none_client, { exact: true })).toBeVisible();
		await expect(
			page.getByRole('link', { name: es.attributes_none_link, exact: true })
		).toBeVisible();
		await expect(page.getByText('Café', { exact: true })).toBeVisible();
	});
});
