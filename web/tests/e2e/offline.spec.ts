// CUJ 6 (design doc item 79): the server is off. The service worker is blocked so Playwright's
// routing sees every request; the API then answers like Cloudflare with the PC off, a 530 HTML
// page without `X-App-Version`, which the app reads as "offline". Pages still load from the
// server: only `/api/` is cut. A cold open with the server off (the service worker's job) stays
// the manual check in the Verification Plan.
import { expect, test } from '@playwright/test';
import es from '../../src/lib/i18n/es.json' with { type: 'json' };
import { at, closePool, insertAppointment, insertClient, resetData } from './seed';
import { draftStored, dueItem, serverOff } from './steps';

test.use({ serviceWorkers: 'block' });

let ana = '';
let todayVisit = '';

test.beforeEach(async ({ page }) => {
	await resetData();
	ana = await insertClient({ name: 'Ana Rojas', phone: '+50688887777' });
	todayVisit = await insertAppointment({
		clientId: ana,
		services: ['Acrílico'],
		startsAt: at(0, '17:00')
	});
	// The 1-day "before" rule makes this one's message due today.
	await insertAppointment({ clientId: ana, services: ['Acrílico'], startsAt: at(1, '17:00') });

	// Online first: the app caches the bootstrap and the time of that refresh.
	await page.goto('/');
	await expect(page.getByRole('link', { name: /Ana Rojas/ })).toBeVisible();
	// Until the first refresh is stored, Hoy asks the server for the day, so Ana's link alone does
	// not prove the cache. The due summary reads only the stored snapshot, which is written with
	// the refresh time in one transaction.
	await expect(page.getByText(es.due_today_summary_one, { exact: true })).toBeVisible();
});

test.afterAll(closePool);

const escape = (text: string): string => text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
/** The banner text for a sync today, at any HH:MM time. */
const BANNER = new RegExp(
	`^${escape(es.offline_banner).replace(
		'\\{when\\}',
		escape(es.synced_today).replace('\\{time\\}', '\\d{2}:\\d{2}')
	)}$`
);

test('offline shows the cached day with the banner', async ({ page }) => {
	await serverOff(page);
	await page.reload();

	await expect(page.getByRole('status').getByText(BANNER)).toBeVisible();
	await expect(page.getByRole('link', { name: /Ana Rojas/ })).toBeVisible();
});

test('offline the banner says why nothing saves, with no note floating over Hoy', async ({
	page
}) => {
	await serverOff(page);
	await page.reload();

	await expect(page.getByRole('status').getByText(es.offline_cannot_save)).toBeVisible();
	await expect(page.getByText(es.offline_cannot_save, { exact: true })).toHaveCount(1);
});

test('the banner clears when the connection comes back, with the app left open', async ({
	page,
	context
}) => {
	await serverOff(page);
	await page.reload();
	const banner = page.getByRole('status').getByText(BANNER);
	await expect(banner).toBeVisible();

	await page.unroute('**/api/**');
	// The phone loses and regains its network: the page gets the `online` event, and no focus.
	await context.setOffline(true);
	await context.setOffline(false);

	await expect(banner).toBeHidden();
});

test('offline keeps the cached due list read-only', async ({ page }) => {
	await page.goto('/enviar');
	await expect(dueItem(page, 'Ana Rojas')).toBeVisible();
	await serverOff(page);
	await page.reload();

	await expect(page.getByRole('status').getByText(BANNER)).toBeVisible();
	const item = dueItem(page, 'Ana Rojas');
	for (const name of [es.due_send, es.due_skip]) {
		const button = item.getByRole('button', { name, exact: true });
		await expect(button).toBeDisabled();
		await expect(button).toHaveAccessibleDescription(es.offline_cannot_save);
	}
});

test('offline the due list says why its buttons are off once, not under every card', async ({
	page
}) => {
	const marta = await insertClient({ name: 'Marta Solano', phone: '+50688886666' });
	await insertAppointment({ clientId: marta, services: ['Acrílico'], startsAt: at(1, '10:00') });
	await page.goto('/enviar');
	await expect(dueItem(page, 'Marta Solano')).toBeVisible();
	await serverOff(page);
	await page.reload();

	await expect(page.getByRole('status').getByText(BANNER)).toBeVisible();
	await expect(dueItem(page, 'Marta Solano')).toBeVisible();
	await expect(page.getByText(es.offline_cannot_save, { exact: true })).toHaveCount(1);
});

test('offline "Agregar foto" is off and says why', async ({ page }) => {
	await page.goto(`/clientas/${ana}`);
	await expect(page.getByText(es.photos_empty_client, { exact: true })).toBeVisible();
	await serverOff(page);
	await page.reload();

	await expect(page.getByRole('status').getByText(BANNER)).toBeVisible();
	const add = page.getByRole('button', { name: es.photo_add });
	await expect(add).toBeDisabled();
	await expect(add).toHaveAccessibleDescription(es.offline_cannot_save);
});

test('CUJ N5: offline "Agendar próxima cita" is off and says why', async ({ page }) => {
	await page.goto(`/citas/${todayVisit}`);
	await expect(page.getByRole('heading', { name: es.appointment_title, level: 1 })).toBeVisible();
	await serverOff(page);
	await page.reload();

	await expect(page.getByRole('status').getByText(BANNER)).toBeVisible();
	const next = page.getByRole('button', { name: es.appointment_next, exact: true });
	await expect(next).toBeDisabled();
	await expect(next).toHaveAccessibleDescription(es.offline_cannot_save);
});

test('a started form is kept as a draft', async ({ page }) => {
	await serverOff(page);
	await page.goto('/clientas/nueva');
	await expect(page.getByRole('status').getByText(BANNER)).toBeVisible();

	await page.getByLabel(es.client_name, { exact: true }).fill('Lucía Rojas');
	await expect.poll(() => draftStored(page, 'clienta')).toBe(true);
	await page.reload();

	await expect(page.getByText(es.draft_restored, { exact: true })).toBeVisible();
	await expect(page.getByLabel(es.client_name, { exact: true })).toHaveValue('Lucía Rojas');
});
