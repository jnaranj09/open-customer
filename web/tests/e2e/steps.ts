// Steps and helpers more than one e2e spec uses. jscpd scans tests/ with threshold 0, so a step two
// specs need lives here, not in both.
import type { Locator, Page } from '@playwright/test';

/** An es.json text with its `{name}` placeholders filled; a missing value throws. */
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
