// Quick replies (reminders-and-replies design, CUJ R11, and the copy of CUJ R6): Enviar →
// Respuestas. The clipboard is stubbed as in quote.spec.ts.
import { expect, test, type Page } from '@playwright/test';
import es from '../../src/lib/i18n/es.json' with { type: 'json' };
import { closePool, insertReply, resetData } from './seed';
import { copied, fill, serverOff, stubClipboard } from './steps';

// As in offline.spec.ts: with the service worker blocked, routing sees every request.
test.use({ serviceWorkers: 'block' });

test.beforeEach(resetData);

test.afterAll(closePool);

/** The reply titles of the first list, in the order shown: the first line of each row's button. */
async function titles(page: Page): Promise<string[]> {
	const rows = await page.getByRole('list').first().getByRole('listitem').all();
	const texts = await Promise.all(rows.map((row) => row.getByRole('button').first().innerText()));
	return texts.map((text) => text.split('\n')[0]?.trim() ?? '');
}

/** Opens a reply's sheet from its row. */
async function openReply(page: Page, title: string): Promise<void> {
	await page.getByRole('button', { name: new RegExp(`^${title} `) }).click();
	await expect(page.getByRole('dialog', { name: title })).toBeVisible();
}

async function newReply(page: Page, title: string, body: string): Promise<void> {
	await page.getByRole('button', { name: es.replies_new }).click();
	const sheet = page.getByRole('dialog', { name: es.replies_new });
	await sheet.getByLabel(es.reply_title, { exact: true }).fill(title);
	await sheet.getByLabel(es.reply_body, { exact: true }).fill(body);
	await sheet.getByRole('button', { name: es.reply_save }).click();
	await expect(page.getByText(es.reply_created)).toBeVisible();
	await expect(sheet).toBeHidden();
}

test('CUJ R11: create, move, edit, archive, restore, search and copy', async ({ page }) => {
	await stubClipboard(page, 'ok');
	await page.goto('/enviar/respuestas');
	await expect(page.getByText(es.replies_empty_title)).toBeVisible();

	await newReply(page, 'Dirección', 'Calle 1, San José');
	await newReply(page, 'Precios', 'Uñas: 20 USD');
	await expect.poll(() => titles(page)).toEqual(['Dirección', 'Precios']);

	await openReply(page, 'Precios');
	await page.getByRole('dialog').getByRole('button', { name: es.option_up }).click();
	await expect(page.getByRole('dialog').getByRole('button', { name: es.option_up })).toBeDisabled();
	await page.getByRole('dialog').getByLabel(es.reply_body, { exact: true }).fill('Uñas: 25 USD');
	await page.getByRole('dialog').getByRole('button', { name: es.reply_save }).click();
	await expect(page.getByText(es.reply_saved)).toBeVisible();
	await expect.poll(() => titles(page)).toEqual(['Precios', 'Dirección']);

	await openReply(page, 'Dirección');
	await page.getByRole('dialog').getByRole('button', { name: es.reply_archive }).click();
	await expect(page.getByText(es.reply_archived)).toBeVisible();
	await expect.poll(() => titles(page)).toEqual(['Precios']);
	await page
		.getByRole('button', { name: fill(es.replies_archived_toggle, { count: '1' }) })
		.click();
	await openReply(page, 'Dirección');
	await page.getByRole('dialog').getByRole('button', { name: es.reply_restore }).click();
	await expect(page.getByText(es.reply_restored)).toBeVisible();

	await page.getByLabel(es.replies_search, { exact: true }).fill('prec');
	await expect.poll(() => titles(page)).toEqual(['Precios']);
	await page.getByRole('button', { name: es.quote_copy, exact: true }).click();
	await expect(page.getByRole('button', { name: es.quote_copied, exact: true })).toBeVisible();
	expect(await copied(page)).toBe('Uñas: 25 USD');

	await page.getByLabel(es.replies_search, { exact: true }).fill('zz');
	await expect(page.getByText(es.replies_no_results)).toBeVisible();
});

test('CUJ R6: offline she finds a reply by a part of its name and copies it', async ({ page }) => {
	await insertReply({ title: 'Dirección', body: 'Calle 1, San José' });
	await insertReply({ title: 'Precios', body: 'Uñas: 20 USD' });
	await stubClipboard(page, 'ok');
	await page.goto('/enviar/respuestas');
	await expect(page.getByRole('button', { name: /^Dirección / })).toBeVisible();

	await serverOff(page);
	await page.reload();
	await page.getByLabel(es.replies_search, { exact: true }).fill('direc');
	await expect.poll(() => titles(page)).toEqual(['Dirección']);
	await page.getByRole('button', { name: es.quote_copy, exact: true }).click();
	await expect(page.getByRole('button', { name: es.quote_copied, exact: true })).toBeVisible();
	expect(await copied(page)).toBe('Calle 1, San José');
});
