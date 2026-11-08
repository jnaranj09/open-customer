// Steps and helpers more than one e2e spec uses. jscpd scans tests/ with threshold 0, so a step two
// specs need lives here, not in both.
import { expect, type Locator, type Page } from '@playwright/test';
import { DB_NAME } from '../../src/lib/cache/db';
import { draftKey } from '../../src/lib/cache/drafts';
import es from '../../src/lib/i18n/es.json' with { type: 'json' };

/** An es.json text with its `{name}` placeholders filled; a missing value throws. Copies `t()` of src/lib/i18n, whose
 *  es.json import (no `type: json`) Playwright's loader rejects. */
export function fill(text: string, params: Record<string, string>): string {
	return text.replace(/\{(\w+)\}/g, (_match, name: string) => {
		const value = params[name];
		if (value === undefined) {
			throw new Error(`fill: no value for {${name}}`);
		}
		return value;
	});
}

/** The due item of `name` on the phone layout: a list card headed by the client's name. */
export function dueItem(page: Page, name: string): Locator {
	return page
		.getByRole('listitem')
		.filter({ has: page.getByRole('heading', { name, exact: true, level: 2 }) });
}

/** Whether the new-record draft of `form` (`cita`, `clienta` or `cotizacion`) is stored on the phone
 *  (a form saves it 500 ms after input). */
export function draftStored(page: Page, form: Parameters<typeof draftKey>[0]): Promise<boolean> {
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
		[DB_NAME, draftKey(form, 'new')] as const
	);
}

/** Every API call answers like Cloudflare when the PC is off. */
export async function serverOff(page: Page): Promise<void> {
	await page.route('**/api/**', (route) =>
		route.fulfill({ status: 530, contentType: 'text/html', body: '<html><body>530</body></html>' })
	);
}

/** Servicios → Mis servicios: archives the service called `name`, or restores it. The
 *  services outlive `resetData`, so a test that archives one restores it in a `finally`. */
export async function setArchived(page: Page, name: string, archive: boolean): Promise<void> {
	await page.goto('/servicios/catalogo');
	await page.getByRole('button', { name: new RegExp(`^${name}`) }).click();
	const sheet = page.getByRole('dialog', { name: es.service_edit_title });
	await sheet
		.getByRole('button', { name: archive ? es.service_archive : es.service_restore })
		.click();
	await expect(page.getByText(archive ? es.service_archived : es.service_restored)).toBeVisible();
}
