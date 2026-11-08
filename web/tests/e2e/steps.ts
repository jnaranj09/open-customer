// Steps and helpers more than one e2e spec uses. jscpd scans tests/ with threshold 0, so a step two
// specs need lives here, not in both.
import type { Locator, Page } from '@playwright/test';
import { DB_NAME } from '../../src/lib/cache/db';
import { draftKey } from '../../src/lib/cache/drafts';

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

/** Whether the new-record draft of `form` is stored on the phone (a form saves it 500 ms after
 *  input). */
export function draftStored(page: Page, form: 'cita' | 'clienta'): Promise<boolean> {
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
