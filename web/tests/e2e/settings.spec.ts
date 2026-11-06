// Settings screens: a toast raised over an open sheet stays usable (BottomSheet makes the page
// inert, the toast is a live layer), and the color picker names its swatches. Builds on the
// "Forma" attribute (options Almendra, Cuadrada) that `setup.spec.ts` creates; each test that
// archives an option brings it back with "Deshacer".
import { expect, test, type Locator, type Page } from '@playwright/test';
import es from '../../src/lib/i18n/es.json' with { type: 'json' };
import { fill } from './steps';

/** Opens the "Forma" sheet, archives "Cuadrada" from its menu and waits for the undo toast. Returns
 *  the sheet and the option's menu button. */
async function archiveCuadrada(page: Page): Promise<{ sheet: Locator; option: Locator }> {
	await page.goto('/ajustes/atributos');
	await page.getByRole('main').getByText('Forma', { exact: true }).click();
	const sheet = page.getByRole('dialog', { name: 'Forma' });
	const option = sheet.getByRole('button', { name: fill(es.option_menu, { label: 'Cuadrada' }) });
	await option.click();
	await sheet.getByRole('button', { name: es.option_archive, exact: true }).click();
	await expect(page.getByText(es.option_archived, { exact: true })).toBeVisible();
	await expect(option).toBeHidden();
	await expect(sheet).toBeVisible();
	return { sheet, option };
}

test('"Deshacer" on a toast stays tappable when a sheet opens over it', async ({ page }) => {
	// She archives an option, closes the sheet and opens it again while the toast still shows: the
	// sheet makes the page inert, and the toast must stay out of that.
	const { sheet, option } = await archiveCuadrada(page);
	await page.keyboard.press('Escape');
	await expect(sheet).toBeHidden();
	await page.getByRole('main').getByText('Forma', { exact: true }).click();
	await expect(sheet).toBeVisible();

	await page.getByRole('button', { name: es.due_undo, exact: true }).click();

	await expect(option).toBeVisible();
});

test('Tab inside an open sheet reaches the toast\'s "Deshacer"', async ({ page }) => {
	const { option } = await archiveCuadrada(page);
	const undo = page.getByRole('button', { name: es.due_undo, exact: true });

	await expect
		.poll(
			async () => {
				await page.keyboard.press('Tab');
				return undo.evaluate((el) => el === document.activeElement);
			},
			{ intervals: [50], timeout: 4000 }
		)
		.toBe(true);
	await page.keyboard.press('Enter');

	await expect(option).toBeVisible();
});

test('the color swatches are named by color, not by hex code', async ({ page }) => {
	await page.goto('/ajustes/negocio');

	const rose = page.getByRole('button', {
		name: fill(es.color_swatch, { name: es.color_name_rose }),
		exact: true
	});
	await expect(rose).toBeVisible();
	await expect(rose).toHaveAttribute('aria-pressed', 'true');
});
