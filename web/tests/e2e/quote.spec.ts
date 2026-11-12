// Stages S3 and S4 of quick quotes (docs/designs/quick-quotes.md, items 22-31): the Cotizar tab of
// Servicios. She picks services, reads the quote, taps Copiar, or turns it into a Nueva cita. It
// builds on the services `setup.spec.ts` creates (Acrílico, Relleno, Pedicure, all priced);
// `resetData` leaves no client. The clipboard is stubbed: the spec reads what was written, and
// three variants refuse the write.
import { expect, test, type Page } from '@playwright/test';
import { DB_NAME } from '../../src/lib/cache/db';
import es from '../../src/lib/i18n/es.json' with { type: 'json' };
import { businessDate, closePool, insertClient, resetData } from './seed';
import {
	copied,
	draftStored,
	failDraftWrite,
	fill,
	serverOff,
	setArchived,
	stubClipboard
} from './steps';

/** The business name `setup.spec.ts` gives the app. */
const HEADING = fill(es.quote_heading_named, { name: 'Estudio Prueba' });

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

test('CUJ R8: a service description is added at the end of the quote', async ({ page }) => {
	await stubClipboard(page, 'ok');
	await page.goto('/servicios');
	await chip(page, 'Acrílico').click();
	await chip(page, 'Relleno').click();
	await copyButton(page).click();
	await expect(page.getByRole('button', { name: es.quote_copied, exact: true })).toBeVisible();
	// No described service: the text is as before, with no blank line.
	expect(await copied(page)).not.toContain('\n\n');

	await page.goto('/servicios/catalogo');
	await page.getByRole('button', { name: /^Acrílico/ }).click();
	const sheet = page.getByRole('dialog', { name: es.service_edit_title });
	await sheet.getByLabel(es.service_description).fill('Incluye limado y esmalte');
	await sheet.getByRole('button', { name: es.service_save }).click();
	await expect(page.getByText(es.service_saved)).toBeVisible();

	await page.goto('/servicios');
	await chip(page, 'Acrílico').click();
	await chip(page, 'Relleno').click();
	await copyButton(page).click();
	await expect(page.getByRole('button', { name: es.quote_copied, exact: true })).toBeVisible();
	expect(await copied(page)).toMatch(
		new RegExp(
			`\\n\\n${fill(es.quote_detail_line, { name: 'Acrílico', description: 'Incluye limado y esmalte' })}$`
		)
	);
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

const convertButton = (page: Page) =>
	page.getByRole('button', { name: es.quote_convert, exact: true });

test('a quoted service archived while Cotizar is open drops out: the quote is empty', async ({
	page,
	context
}) => {
	await page.clock.install();
	await page.goto('/servicios');
	await chip(page, 'Acrílico').click();
	await expect(copyButton(page)).toBeVisible();
	const other = await context.newPage();

	try {
		await setArchived(other, 'Acrílico', true);
		// The open page learns of it on its next focus refresh, which waits for the last one to be
		// 30 s old.
		await page.clock.fastForward(60_000);
		await page.evaluate(() => {
			window.dispatchEvent(new Event('focus'));
		});

		await expect(page.getByText(es.quote_empty)).toBeVisible();
		await expect(copyButton(page)).toHaveCount(0);
		await expect(convertButton(page)).toHaveCount(0);
	} finally {
		await setArchived(other, 'Acrílico', false);
	}
});

/** The services chips of Nueva cita (the same group as Cotizar's phone picker). */
const picked = (page: Page, name: string) =>
	page
		.getByRole('group', { name: es.appointment_services })
		.getByRole('button', { name: new RegExp(`^${name}`) });

/** The `lines` of a stored draft (`cotizacion:new` or `cita:new`), in stored order. The chips of
 *  Nueva cita follow the catalog, so only the draft shows the order she picked in. */
const draftLines = (page: Page, key: string) =>
	page.evaluate(
		([name, k]) =>
			new Promise<{ service_id: string; count: number }[]>((resolve, reject) => {
				const open = indexedDB.open(name);
				open.onerror = () => {
					reject(new Error('indexedDB.open failed'));
				};
				open.onsuccess = () => {
					const req = open.result.transaction('drafts').objectStore('drafts').get(k);
					req.onerror = () => {
						reject(new Error('drafts read failed'));
					};
					req.onsuccess = () => {
						open.result.close();
						const row = req.result as { value: { lines: { service_id: string; count: number }[] } };
						resolve(row.value.lines);
					};
				};
			}),
		[DB_NAME, key] as const
	);

const removeButton = (page: Page, name: string) =>
	page.getByRole('button', { name: fill(es.appointment_service_remove, { name }) });

test('CUJ Q2: convert opens Nueva cita with the services and today, and saving lands on Hoy', async ({
	page
}) => {
	await insertClient({ name: 'Ana Rojas', phone: '+50688887777' });
	await page.goto('/servicios');
	// Picked against the catalog order (Acrílico, Relleno, Pedicure): Relleno twice, then Acrílico.
	await chip(page, 'Relleno').click();
	await chip(page, 'Relleno').click();
	await chip(page, 'Acrílico').click();
	await expect.poll(() => draftStored(page, 'cotizacion')).toBe(true);
	await expect.poll(async () => (await draftLines(page, 'cotizacion:new')).length).toBe(2);
	const quoteLines = await draftLines(page, 'cotizacion:new');

	await convertButton(page).click();

	await expect(page).toHaveURL(/\/citas\/nueva$/);
	// Same services, in the order she picked them (the counts tell the two apart).
	expect(await draftLines(page, 'cita:new')).toEqual(quoteLines);
	expect(quoteLines.map((l) => l.count)).toEqual([2, 1]);
	await expect(page.getByRole('heading', { name: es.appointment_new, level: 1 })).toBeVisible();
	await expect(picked(page, 'Relleno')).toContainText('×2');
	await expect(removeButton(page, 'Acrílico')).toBeVisible();
	await expect(removeButton(page, 'Pedicure')).toHaveCount(0);
	await expect(page.getByLabel(es.appointment_day, { exact: true })).toHaveValue(businessDate(0));
	await expect(page.getByText(es.draft_restored)).toBeVisible();
	// The quote went with the hand-off.
	await expect.poll(() => draftStored(page, 'cotizacion')).toBe(false);

	await page.getByLabel(es.appointment_client, { exact: true }).fill('ana');
	await page.getByRole('button', { name: 'Ana Rojas', exact: true }).click();
	await page.getByLabel(es.appointment_time, { exact: true }).fill('10:00');
	await page.getByRole('button', { name: es.appointment_save }).click();

	await expect(page.getByRole('heading', { name: es.today_title, level: 1 })).toBeVisible();
	await expect(page.getByRole('link', { name: /Ana Rojas/ })).toBeVisible();
	await expect.poll(() => draftStored(page, 'cita')).toBe(false);
	await page.goto('/servicios');
	await expect(page.getByText(es.quote_empty)).toBeVisible();
});

test.describe('with an unsaved Nueva cita', () => {
	test.beforeEach(async ({ page }) => {
		await insertClient({ name: 'Ana Rojas', phone: '+50688887777' });
		await page.goto('/citas/nueva');
		await page.getByLabel(es.appointment_client, { exact: true }).fill('ana');
		await page.getByRole('button', { name: 'Ana Rojas', exact: true }).click();
		await page.getByRole('button', { name: 'Pedicure', exact: true }).click();
		await expect.poll(() => draftStored(page, 'cita')).toBe(true);
		await page.goto('/servicios');
		await chip(page, 'Acrílico').click();
		await convertButton(page).click();
		await expect(page.getByText(es.draft_replace_prompt)).toBeVisible();
	});

	test('CUJ Q3: Cancelar keeps the quote and the unsaved booking', async ({ page }) => {
		await page.getByRole('button', { name: es.cancel, exact: true }).click();

		await expect(page.getByText(es.draft_replace_prompt)).toHaveCount(0);
		await expect(quoteBlock(page)).toContainText('Acrílico');
		await expect(page).toHaveURL(/\/servicios$/);
		await page.goto('/citas/nueva');
		await expect(removeButton(page, 'Pedicure')).toBeVisible();
		await expect(removeButton(page, 'Acrílico')).toHaveCount(0);
		await expect(page.getByText('Ana Rojas')).toBeVisible();
	});

	test('CUJ Q3: Reemplazar puts the quote into Nueva cita', async ({ page }) => {
		await page.getByRole('button', { name: es.draft_replace, exact: true }).click();

		await expect(page).toHaveURL(/\/citas\/nueva$/);
		await expect(removeButton(page, 'Acrílico')).toBeVisible();
		await expect(removeButton(page, 'Pedicure')).toHaveCount(0);
		await expect(page.getByText('Ana Rojas')).toHaveCount(0);
		await expect.poll(() => draftStored(page, 'cotizacion')).toBe(false);
	});
});

test('a failed write says so and keeps the quote', async ({ page }) => {
	const logs: string[] = [];
	page.on('console', (message) => {
		if (message.type() === 'warning') {
			logs.push(message.text());
		}
	});
	// Only the hand-off writes `cita:new` from this page; the quote's own draft is untouched.
	await failDraftWrite(page, 'cita:new');
	await page.goto('/servicios');
	await chip(page, 'Acrílico').click();

	await convertButton(page).click();

	await expect(page.getByRole('alert').filter({ hasText: es.cita_handoff_failed })).toBeVisible();
	await expect(page).toHaveURL(/\/servicios$/);
	await expect(quoteBlock(page)).toContainText('Acrílico');
	expect(logs).toContain('code=cita_handoff_failed');
	expect(logs.join('\n')).not.toContain('Acrílico');
	await expect.poll(() => draftStored(page, 'cotizacion')).toBe(true);

	// The message goes with her next pick.
	await chip(page, 'Relleno').click();
	await expect(page.getByRole('alert').filter({ hasText: es.cita_handoff_failed })).toHaveCount(0);
});

test('convert works offline', async ({ page }) => {
	await page.goto('/servicios');
	await expect(chip(page, 'Acrílico')).toBeVisible();
	await serverOff(page);
	await page.reload();
	await expect(page.getByRole('status').getByText(es.offline_cannot_save)).toBeVisible();
	await chip(page, 'Acrílico').click();

	await convertButton(page).click();

	await expect(page).toHaveURL(/\/citas\/nueva$/);
	await expect(removeButton(page, 'Acrílico')).toBeVisible();
	await expect(page.getByLabel(es.appointment_day, { exact: true })).toHaveValue(businessDate(0));
});
