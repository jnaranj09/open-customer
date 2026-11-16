// Mobile polish S1 (docs/designs/mobile-polish.md): on the phone every "create" action is the
// floating "+" at the bottom right, with the same name the top button has on wide. The "+" opens
// the form or the sheet, there is no button at the top, and offline it fades and does nothing
// while the banner says why. On the master-detail screens it is not drawn over the form. On wide
// the button stays at the top and there is no "+". Hoy and Mis servicios have their own specs.
// "Nueva regla" needs an active template: the setup's templates are there (resetData keeps them).
import { expect, test, type Locator, type Page } from '@playwright/test';
import es from '../../src/lib/i18n/es.json' with { type: 'json' };
import { closePool, insertClient, resetData } from './seed';
import { serverOff } from './steps';

test.use({ serviceWorkers: 'block' });

test.beforeEach(async () => {
	await resetData();
	await insertClient({ name: 'Ana Rojas', phone: '+50688887777' });
});

test.afterAll(closePool);

interface Screen {
	name: string;
	path: string;
	label: string;
	role: 'link' | 'button';
	opened: (page: Page) => Locator;
}

const SCREENS: Screen[] = [
	{
		name: 'Clientas',
		path: '/clientas',
		label: es.clients_new,
		role: 'link',
		opened: (page) => page.getByRole('heading', { name: es.client_new_title, level: 1 })
	},
	{
		name: 'Plantillas',
		path: '/ajustes/plantillas',
		label: es.templates_new,
		role: 'link',
		opened: (page) => page.getByRole('heading', { name: es.templates_new_title, level: 2 })
	},
	{
		name: 'Reglas',
		path: '/ajustes/recordatorios',
		label: es.rules_new,
		role: 'button',
		opened: (page) => page.getByRole('dialog', { name: es.rule_new_title })
	},
	{
		name: 'Atributos',
		path: '/ajustes/atributos',
		label: es.attributes_new,
		role: 'button',
		opened: (page) => page.getByRole('dialog', { name: es.attribute_new_title })
	},
	{
		name: 'Respuestas',
		path: '/enviar/respuestas',
		label: es.replies_new,
		role: 'button',
		opened: (page) => page.getByRole('dialog', { name: es.replies_new })
	}
];

/** The master-detail screens: their form, where the "+" is not drawn. */
const FORMS = [
	{ name: 'Clientas', path: '/clientas/nueva', label: es.clients_new, title: es.client_new_title },
	{
		name: 'Plantillas',
		path: '/ajustes/plantillas/nueva',
		label: es.templates_new,
		title: es.templates_new_title
	}
];

/** Whether the control sits in a fixed layer (the floating "+"), not in the page flow. */
function isFloating(control: Locator): Promise<boolean> {
	return control.evaluate((el) => {
		for (let node: Element | null = el; node !== null; node = node.parentElement) {
			if (getComputedStyle(node).position === 'fixed') {
				return true;
			}
		}
		return false;
	});
}

for (const screen of SCREENS) {
	test.describe(screen.name, () => {
		test('on the phone the create action is the floating "+" and opens the form', async ({
			page
		}) => {
			await page.goto(screen.path);
			const add = page.getByRole(screen.role, { name: screen.label, exact: true });

			await expect(add).toHaveCount(1);
			await expect(add).toBeVisible();
			expect(await isFloating(add)).toBe(true);
			await add.click();

			await expect(screen.opened(page)).toBeVisible();
		});

		test('offline the "+" is faded and does nothing', async ({ page }) => {
			await page.goto(screen.path);
			await expect(page.getByRole(screen.role, { name: screen.label, exact: true })).toBeVisible();
			await serverOff(page);
			await page.reload();

			await expect(page.getByRole('status').getByText(es.offline_cannot_save)).toBeVisible();
			await expect(page.getByRole('img', { name: screen.label, exact: true })).toBeVisible();
			await expect(page.getByRole(screen.role, { name: screen.label, exact: true })).toHaveCount(0);
		});

		test.describe('wide', () => {
			test.use({ viewport: { width: 1440, height: 900 } });

			test('the create action stays a button at the top, with no "+"', async ({ page }) => {
				await page.goto(screen.path);
				const add = page.getByRole(screen.role, { name: screen.label, exact: true });

				await expect(add).toHaveCount(1);
				await expect(add).toBeVisible();
				expect(await isFloating(add)).toBe(false);
				await expect(page.getByRole('img', { name: screen.label, exact: true })).toHaveCount(0);
			});
		});
	});
}

for (const form of FORMS) {
	test(`${form.name}: on the phone the "+" is not drawn over the form`, async ({ page }) => {
		await page.goto(form.path);
		await expect(page.getByRole('heading', { name: form.title })).toBeVisible();

		await expect(page.getByRole('link', { name: form.label, exact: true })).toHaveCount(0);
	});
}
