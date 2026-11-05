// CUJ 6 (design doc item 79): the server is off. The service worker is blocked so Playwright's
// routing sees every request; the API then answers like Cloudflare with the PC off, a 530 HTML
// page without `X-App-Version`, which the app reads as "offline". Pages still load from the
// server: only `/api/` is cut. A cold open with the server off (the service worker's job) stays
// the manual check in the Verification Plan.
import { expect, test, type Page } from '@playwright/test';
import es from '../../src/lib/i18n/es.json' with { type: 'json' };
import { DB_NAME } from '../../src/lib/cache/db';
import { draftKey } from '../../src/lib/cache/drafts';
import { at, closePool, insertAppointment, insertClient, resetData } from './seed';
import { dueItem } from './steps';

test.use({ serviceWorkers: 'block' });

test.beforeEach(async ({ page }) => {
	await resetData();
	const ana = await insertClient({ name: 'Ana Rojas', phone: '+50688887777' });
	await insertAppointment({ clientId: ana, services: ['Acrílico'], startsAt: at(0, '17:00') });
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

/** The banner text with any HH:MM time. */
const BANNER = new RegExp(
	`^${es.offline_banner.replace(/[.*+?^${}()|[\]\\]/g, '\\$&').replace('\\{time\\}', '\\d{2}:\\d{2}')}$`
);

/** Every API call answers like Cloudflare when the PC is off. */
async function serverOff(page: Page): Promise<void> {
	await page.route('**/api/**', (route) =>
		route.fulfill({ status: 530, contentType: 'text/html', body: '<html><body>530</body></html>' })
	);
}

/** Whether the new-client draft is stored on the phone (the form saves it 500 ms after typing). */
function draftStored(page: Page): Promise<boolean> {
	return page.evaluate(
		([name, key]) =>
			new Promise<boolean>((resolve, reject) => {
				const open = indexedDB.open(name);
				open.onerror = () => {
					reject(new Error('indexedDB.open failed'));
				};
				open.onsuccess = () => {
					const req = open.result.transaction('drafts').objectStore('drafts').get(key);
					req.onerror = () => {
						reject(new Error('drafts read failed'));
					};
					req.onsuccess = () => {
						open.result.close();
						resolve(req.result !== undefined);
					};
				};
			}),
		[DB_NAME, draftKey('clienta', 'new')] as const
	);
}

test('offline shows the cached day with the banner', async ({ page }) => {
	await serverOff(page);
	await page.reload();

	await expect(page.getByRole('status').getByText(BANNER)).toBeVisible();
	await expect(page.getByRole('link', { name: /Ana Rojas/ })).toBeVisible();
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

test('a started form is kept as a draft', async ({ page }) => {
	await serverOff(page);
	await page.goto('/clientas/nueva');
	await expect(page.getByRole('status').getByText(BANNER)).toBeVisible();

	await page.getByLabel(es.client_name, { exact: true }).fill('Lucía Rojas');
	await expect.poll(() => draftStored(page)).toBe(true);
	await page.reload();

	await expect(page.getByText(es.draft_restored, { exact: true })).toBeVisible();
	await expect(page.getByLabel(es.client_name, { exact: true })).toHaveValue('Lucía Rojas');
});
