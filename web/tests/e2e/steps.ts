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

/** From the next page load on, writing the draft `key` (`cita:new`) fails like a full phone: the
 *  IndexedDB `put` throws a QuotaExceededError for that key only. Call it before the first `goto`. */
export async function failDraftWrite(page: Page, key: string): Promise<void> {
	await page.addInitScript((failing) => {
		const put = Object.getOwnPropertyDescriptor(IDBObjectStore.prototype, 'put')?.value as (
			this: IDBObjectStore,
			value: unknown,
			key?: IDBValidKey
		) => IDBRequest<IDBValidKey>;
		IDBObjectStore.prototype.put = function (value: unknown, key?: IDBValidKey) {
			if ((value as { key?: string }).key === failing) {
				throw new DOMException('full', 'QuotaExceededError');
			}
			return put.call(this, value, key);
		};
	}, key);
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

type ClipboardMode = 'ok' | 'missing' | 'throws' | 'rejects';

/** Replaces `navigator.clipboard` before the app loads. `ok` records the written text on
 *  `window.copied`; the others fail the way iOS can: no API, a throw at once, a rejected promise. */
export async function stubClipboard(page: Page, mode: ClipboardMode): Promise<void> {
	await page.addInitScript((m: ClipboardMode) => {
		const win = window as unknown as { copied: string | null };
		win.copied = null;
		const writeText = (text: string): Promise<void> => {
			if (m === 'throws') {
				throw new DOMException('refused', 'NotAllowedError');
			}
			if (m === 'rejects') {
				return Promise.reject(new DOMException('refused', 'NotAllowedError'));
			}
			win.copied = text;
			return Promise.resolve();
		};
		Object.defineProperty(navigator, 'clipboard', {
			configurable: true,
			value: m === 'missing' ? undefined : { writeText }
		});
	}, mode);
}

export const copied = (page: Page): Promise<string | null> =>
	page.evaluate(() => (window as unknown as { copied: string | null }).copied);

/** The ways `stubClipboard` refuses a copy. */
export const COPY_REFUSALS = ['missing', 'throws', 'rejects'] as const;

/** The app's toast (`Toast.svelte`, a `status`) while it says exactly `text`. */
const toast = (page: Page, text: string): Locator =>
	page.getByRole('status').filter({ has: page.getByText(text, { exact: true }) });

/** Taps the one "Copiar" in `scope` with the clipboard stubbed `ok`: the button and the toast both
 *  say "Copiado". Returns what the clipboard got. */
export async function copyFrom(page: Page, scope: Page | Locator): Promise<string> {
	await scope.getByRole('button', { name: es.copy, exact: true }).click();
	await expect(scope.getByRole('button', { name: es.copied, exact: true })).toBeVisible();
	await expect(toast(page, es.copied)).toBeVisible();
	const text = await copied(page);
	expect(text).not.toBeNull();
	return text ?? '';
}

/** Taps the one "Copiar" in `scope` with a refusing clipboard: the toast says `copy_failed`, the
 *  button never says "Copiado", and no message is left on the page. */
export async function copyRefused(page: Page, scope: Page | Locator): Promise<void> {
	await scope.getByRole('button', { name: es.copy, exact: true }).click();
	await expect(toast(page, es.copy_failed)).toBeVisible();
	await expect(scope.getByRole('button', { name: es.copied })).toHaveCount(0);
	await expect(page.getByRole('alert')).toHaveCount(0);
}
