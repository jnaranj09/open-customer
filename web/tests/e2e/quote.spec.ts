// Stage S3 of quick quotes (docs/designs/quick-quotes.md, items 22-27): the Cotizar tab of Servicios.
// She picks services, reads the quote and taps Copiar. It builds on the services `setup.spec.ts`
// creates (Acrílico, Relleno, Pedicure, all priced); `resetData` leaves no client. The clipboard is
// stubbed: the spec reads what was written, and three variants refuse the write.
import { expect, test, type Page } from '@playwright/test';
import es from '../../src/lib/i18n/es.json' with { type: 'json' };
import { closePool, resetData } from './seed';
import { draftStored, fill, serverOff } from './steps';

/** The business name `setup.spec.ts` gives the app. */
const HEADING = fill(es.quote_heading_named, { name: 'Estudio Prueba' });

type Clipboard = 'ok' | 'missing' | 'throws' | 'rejects';

/** Replaces `navigator.clipboard` before the app loads. `ok` records the written text on
 *  `window.copied`; the others fail the way iOS can: no API, a throw at once, a rejected promise. */
async function stubClipboard(page: Page, mode: Clipboard): Promise<void> {
	await page.addInitScript((m: Clipboard) => {
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

const copied = (page: Page): Promise<string | null> =>
	page.evaluate(() => (window as unknown as { copied: string | null }).copied);

/** The quote text block, found by its heading line. */
const quoteBlock = (page: Page) => page.getByText(HEADING);

/** The phone's service chips of Cotizar. */
const chip = (page: Page, name: string) =>
	page
		.getByRole('group', { name: es.appointment_services })
		.getByRole('button', { name: new RegExp(`^${name}`) });

const copyButton = (page: Page) => page.getByRole('button', { name: es.quote_copy, exact: true });

// As in offline.spec.ts: with the service worker blocked, Playwright's routing sees every request,
// so `serverOff` really cuts the API.
test.use({ serviceWorkers: 'block' });

test.beforeEach(resetData);

test.afterAll(closePool);

test('tapping Servicios lands on Cotizar, which asks for services to be picked', async ({
	page
}) => {
	await page.goto('/');

	await page
		.getByRole('navigation', { name: es.nav_label })
		.getByRole('link', { name: es.nav_services, exact: true })
		.click();

	await expect(page).toHaveURL(/\/servicios$/);
	await expect(page.getByRole('heading', { name: es.services_title, level: 1 })).toBeVisible();
	const tabs = page.getByRole('navigation', { name: es.services_tabs_label });
	await expect(tabs.getByRole('link', { name: es.services_tab_quote })).toHaveAttribute(
		'aria-current',
		'page'
	);
	await expect(tabs.getByRole('link', { name: es.services_tab_catalog })).not.toHaveAttribute(
		'aria-current',
		'page'
	);
	await expect(page.getByText(es.quote_empty)).toBeVisible();
	await expect(copyButton(page)).toHaveCount(0);
	await expect(page.getByRole('button', { name: es.quote_new })).toHaveCount(0);
});

test('CUJ Q1: she picks services, copies the quote and the button says Copiado', async ({
	page
}) => {
	await stubClipboard(page, 'ok');
	await page.goto('/servicios');
	await chip(page, 'Acrílico').click();
	await chip(page, 'Relleno').click();
	await chip(page, 'Relleno').click();

	const text = await quoteBlock(page).textContent();
	expect(text).toContain('Acrílico');
	expect(text).toContain('Relleno ×2');
	expect(text).toContain('c/u');
	await copyButton(page).click();

	await expect(page.getByRole('button', { name: es.quote_copied, exact: true })).toBeVisible();
	expect(await copied(page)).toBe(text);
	// The button's own label is the feedback (no toast), and it goes back.
	await expect(copyButton(page)).toBeVisible();
});

for (const mode of ['missing', 'throws', 'rejects'] as const) {
	test(`the phone refuses the copy (${mode}): a message says to press and hold`, async ({
		page
	}) => {
		const logs: string[] = [];
		page.on('console', (message) => {
			if (message.type() === 'warning') {
				logs.push(message.text());
			}
		});
		await stubClipboard(page, mode);
		await page.goto('/servicios');
		await chip(page, 'Acrílico').click();

		await copyButton(page).click();

		await expect(page.getByRole('alert').filter({ hasText: es.quote_copy_failed })).toBeVisible();
		await expect(page.getByRole('button', { name: es.quote_copied })).toHaveCount(0);
		expect(logs).toContain('code=clipboard_failed');
		expect(logs.join('\n')).not.toContain('Acrílico');

		// The message goes with her next pick.
		await chip(page, 'Relleno').click();
		await expect(page.getByRole('alert').filter({ hasText: es.quote_copy_failed })).toHaveCount(0);
	});
}

test('the quote survives a change of tab and Nueva cotización empties it', async ({ page }) => {
	await page.goto('/servicios');
	await chip(page, 'Acrílico').click();
	await expect.poll(() => draftStored(page, 'cotizacion')).toBe(true);
	const tabs = page.getByRole('navigation', { name: es.services_tabs_label });

	await tabs.getByRole('link', { name: es.services_tab_catalog }).click();
	await expect(page).toHaveURL(/\/servicios\/catalogo$/);
	await tabs.getByRole('link', { name: es.services_tab_quote }).click();
	await expect(quoteBlock(page)).toContainText('Acrílico');

	await page.getByRole('button', { name: es.quote_new }).click();
	await expect(page.getByText(es.quote_empty)).toBeVisible();
	await expect.poll(() => draftStored(page, 'cotizacion')).toBe(false);

	await tabs.getByRole('link', { name: es.services_tab_catalog }).click();
	await tabs.getByRole('link', { name: es.services_tab_quote }).click();
	await expect(page.getByText(es.quote_empty)).toBeVisible();
	await expect(page.getByText(es.draft_restored)).toHaveCount(0);
});

test('offline the quote builds and copies', async ({ page }) => {
	await stubClipboard(page, 'ok');
	// Online first so the app caches the catalog.
	await page.goto('/servicios');
	await expect(chip(page, 'Acrílico')).toBeVisible();
	await serverOff(page);
	await page.reload();
	// The offline banner always carries why nothing saves, whatever it says about the last sync.
	await expect(page.getByRole('status').getByText(es.offline_cannot_save)).toBeVisible();
	await chip(page, 'Acrílico').click();

	await copyButton(page).click();

	await expect(page.getByRole('button', { name: es.quote_copied, exact: true })).toBeVisible();
	expect(await copied(page)).toBe(await quoteBlock(page).textContent());
});

test.describe('wide', () => {
	test.use({ viewport: { width: 1440, height: 900 } });

	test('she picks with the table stepper and copies from the Cotización panel', async ({
		page
	}) => {
		await stubClipboard(page, 'ok');
		await page.goto('/servicios');
		const add = page.getByRole('button', {
			name: fill(es.quote_service_add, { name: 'Acrílico' })
		});
		const remove = page.getByRole('button', {
			name: fill(es.appointment_service_remove, { name: 'Acrílico' })
		});
		await expect(remove).toHaveCount(0);

		await add.click();
		await add.click();
		await expect(quoteBlock(page)).toContainText('Acrílico ×2');
		await remove.click();

		const text = await quoteBlock(page).textContent();
		expect(text).toContain('Acrílico');
		expect(text).not.toContain('×2');
		await copyButton(page).click();
		await expect(page.getByRole('button', { name: es.quote_copied, exact: true })).toBeVisible();
		expect(await copied(page)).toBe(text);
	});
});
