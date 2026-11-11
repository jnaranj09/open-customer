// Moving between screens and keeping what she typed: Back after a new form (M4), the appointment's
// back arrow (M7), a typed amount through a status change (M8), the archived list after a restore
// (M9), the client's history after "Ver todas las citas" (L10), and the tabs of Enviar and of
// Ajustes → Recordatorios with their old URLs (reminders-and-replies CUJ R12).
import { expect, test, type Page } from '@playwright/test';
import es from '../../src/lib/i18n/es.json' with { type: 'json' };
import { at, businessDate, closePool, insertAppointment, insertClient, resetData } from './seed';

let anaId = '';

test.beforeEach(async () => {
	await resetData();
	anaId = await insertClient({ name: 'Ana Rojas', phone: '+50688887777' });
});

test.afterAll(closePool);

/** The five places, in order, as the nav shows them: the tab bar names Enviar by its short label. */
const PLACES = [
	{ tab: es.nav_today, row: es.nav_today },
	{ tab: es.tab_send, row: es.nav_to_send },
	{ tab: es.nav_clients, row: es.nav_clients },
	{ tab: es.nav_services, row: es.nav_services },
	{ tab: es.nav_settings, row: es.nav_settings }
];

async function placeNames(page: Page): Promise<string[]> {
	await page.goto('/clientas');
	const links = page.getByRole('navigation', { name: es.nav_label }).getByRole('link');
	await expect(links).toHaveCount(PLACES.length);
	// The due count of Enviar is a badge inside its link: drop it.
	const texts = await links.allTextContents();
	return texts.map((text) => text.trim().replace(/\s*\d+$/, ''));
}

test('the five places, in order', async ({ page }) => {
	const names = await placeNames(page);

	expect(names).toEqual(PLACES.map((place) => place.tab));
});

test.describe('five places wide', () => {
	test.use({ viewport: { width: 1440, height: 900 } });

	test('the five places, in order', async ({ page }) => {
		const names = await placeNames(page);

		expect(names).toEqual(PLACES.map((place) => place.row));
	});
});

test('Back after saving a new client does not open the empty form again', async ({ page }) => {
	await page.goto('/clientas');
	await page.getByRole('link', { name: es.clients_new }).first().click();
	await page.getByLabel(es.client_name, { exact: true }).fill('Lucía Mora');
	await page.getByRole('button', { name: es.client_save }).click();
	// On the phone, saving goes back to the list.
	await expect(page.getByRole('link', { name: /Lucía Mora/ })).toBeVisible();

	await page.goBack();

	await expect(page.getByRole('heading', { name: es.nav_clients, level: 1 })).toBeVisible();
	await expect(page.getByRole('heading', { name: es.client_new_title })).toHaveCount(0);
});

test('Back after cancelling a new appointment does not open the empty form again', async ({
	page
}) => {
	await page.goto('/');
	await page.getByRole('link', { name: es.appointment_new }).click();
	await expect(page.getByRole('heading', { name: es.appointment_new, level: 1 })).toBeVisible();
	await page.getByRole('button', { name: es.cancel, exact: true }).click();
	await expect(page.getByRole('heading', { name: es.today_title, level: 1 })).toBeVisible();

	await page.goBack();

	await expect(page.getByRole('heading', { name: es.appointment_new, level: 1 })).toHaveCount(0);
});

/** The other ways out of a new form (M4): each one, then Back must not show the form again. */
const LEAVE_NEW_FORM: {
	name: string;
	/** Opens the form and leaves it; returns the form's heading. */
	leave: (page: Page) => Promise<{ name: string; level: number }>;
}[] = [
	{
		name: 'saving a new template',
		leave: async (page) => {
			await page.goto('/ajustes/plantillas/nueva');
			// "Zz": sorts after the setup templates, which other specs open first.
			await page.getByLabel(es.template_name, { exact: true }).fill('Zz plantilla de prueba');
			await page.getByRole('textbox', { name: es.template_message }).click();
			await page.keyboard.type('Hola');
			await page.getByRole('button', { name: es.template_save }).click();
			await expect(
				page.getByRole('heading', { name: 'Zz plantilla de prueba', level: 2 })
			).toBeVisible();
			return { name: es.templates_new_title, level: 2 };
		}
	},
	{
		name: 'cancelling a new template',
		leave: async (page) => {
			await page.goto('/ajustes/plantillas');
			await page.getByRole('link', { name: es.templates_new }).click();
			await expect(
				page.getByRole('heading', { name: es.templates_new_title, level: 2 })
			).toBeVisible();
			await page.getByRole('button', { name: es.cancel, exact: true }).click();
			await expect(page.getByRole('heading', { name: es.settings_rules, level: 1 })).toBeVisible();
			return { name: es.templates_new_title, level: 2 };
		}
	},
	{
		name: 'saving a new appointment',
		leave: async (page) => {
			await page.goto('/');
			await page.getByRole('link', { name: es.appointment_new }).click();
			await page.getByLabel(es.appointment_client, { exact: true }).fill('ana');
			await page.getByRole('button', { name: 'Ana Rojas', exact: true }).click();
			await page.getByRole('button', { name: 'Acrílico', exact: true }).click();
			await page.getByLabel(es.appointment_day, { exact: true }).fill(businessDate(1));
			await page.getByLabel(es.appointment_time, { exact: true }).fill('09:00');
			await page.getByRole('button', { name: es.appointment_save }).click();
			await expect(page.getByRole('heading', { name: es.today_title, level: 1 })).toBeVisible();
			return { name: es.appointment_new, level: 1 };
		}
	},
	{
		name: 'cancelling a new client',
		leave: async (page) => {
			await page.goto('/clientas');
			await page.getByRole('link', { name: es.clients_new }).first().click();
			await expect(
				page.getByRole('heading', { name: es.client_new_title, level: 1 })
			).toBeVisible();
			await page.getByRole('button', { name: es.cancel, exact: true }).click();
			await expect(page.getByRole('heading', { name: es.nav_clients, level: 1 })).toBeVisible();
			return { name: es.client_new_title, level: 1 };
		}
	}
];

for (const way of LEAVE_NEW_FORM) {
	test(`Back after ${way.name} does not open the empty form again`, async ({ page }) => {
		const heading = await way.leave(page);

		await page.goBack();

		await expect(page.getByRole('heading', heading)).toHaveCount(0);
	});
}

test("the appointment's back arrow returns to the client page she came from", async ({ page }) => {
	await insertAppointment({ clientId: anaId, services: ['Acrílico'], startsAt: at(1, '10:00') });
	await page.goto(`/clientas/${anaId}`);
	await page.getByRole('link', { name: /10:00/ }).click();
	await expect(page.getByRole('heading', { name: es.appointment_title, level: 1 })).toBeVisible();

	await page.getByRole('button', { name: es.appointment_back }).click();

	await expect(page.getByRole('heading', { name: 'Ana Rojas', level: 1 })).toBeVisible();
});

test("the appointment's back arrow goes to Hoy when the appointment was opened directly", async ({
	page
}) => {
	const appointment = await insertAppointment({
		clientId: anaId,
		services: ['Acrílico'],
		startsAt: at(1, '10:00')
	});
	await page.goto(`/citas/${appointment}`);
	await expect(page.getByRole('heading', { name: es.appointment_title, level: 1 })).toBeVisible();

	await page.getByRole('link', { name: es.appointment_back }).click();

	await expect(page.getByRole('heading', { name: es.today_title, level: 1 })).toBeVisible();
});

test('a typed amount paid stays when she changes the status', async ({ page }) => {
	const appointment = await insertAppointment({
		clientId: anaId,
		services: ['Acrílico'],
		startsAt: at(1, '10:00')
	});
	await page.goto(`/citas/${appointment}`);
	const paid = page.getByLabel(es.appointment_paid, { exact: true });
	await paid.fill('12');

	await page.getByRole('button', { name: es.appointment_confirm, exact: true }).click();

	await expect(page.getByText(es.appointment_updated, { exact: true })).toBeVisible();
	await expect(paid).toHaveValue('12');
});

test('"Ver todas las citas" for a client with none says she has no appointments', async ({
	page
}) => {
	await page.goto(`/clientas/${anaId}`);
	await expect(page.getByText(es.client_history_empty, { exact: true })).toBeVisible();

	await page.getByRole('button', { name: es.client_history_more }).click();

	await expect(page.getByRole('button', { name: es.client_history_more })).toHaveCount(0);
	await expect(page.getByText(es.client_history_empty, { exact: true })).toBeVisible();
});

test.describe('wide', () => {
	test.use({ viewport: { width: 1440, height: 900 } });

	test('the archived list drops a client once she is restored', async ({ page }) => {
		await page.goto(`/clientas/${anaId}`);
		await page.getByRole('button', { name: es.client_archive }).click();
		await page.getByRole('dialog').getByRole('button', { name: es.client_archive }).click();
		await expect(page.getByText(es.client_archived_note, { exact: true })).toBeVisible();
		await page.getByRole('button', { name: es.clients_show_archived }).click();
		const list = page.getByRole('main').getByRole('list').first();
		await expect(list.getByRole('link', { name: /Ana Rojas/ })).toBeVisible();

		await page.getByRole('button', { name: es.client_restore }).click();

		await expect(page.getByText(es.client_archived_note, { exact: true })).toBeHidden();
		await expect(list.getByRole('link', { name: /Ana Rojas/ })).toHaveCount(0);
		await expect(page.getByText(es.clients_archived_empty, { exact: true })).toBeVisible();
	});
});

test('CUJ R12: the old rule and template URLs open their tab inside Recordatorios', async ({
	page
}) => {
	const sections = page.getByRole('navigation', { name: es.settings_sections });
	const tabs = page.getByRole('navigation', { name: es.rules_tabs_label });
	const entry = sections.getByRole('link', { name: es.settings_rules, exact: true });

	await page.goto('/ajustes/plantillas');
	await expect(tabs.getByRole('link', { name: es.tab_templates })).toHaveAttribute(
		'aria-current',
		'page'
	);
	await expect(entry).toHaveAttribute('aria-current', 'true');
	await expect(sections.getByRole('link', { name: es.tab_templates, exact: true })).toHaveCount(0);

	await page.goto('/ajustes/recordatorios');
	await expect(tabs.getByRole('link', { name: es.tab_rules })).toHaveAttribute(
		'aria-current',
		'page'
	);
	await expect(entry).toHaveAttribute('aria-current', 'true');

	await tabs.getByRole('link', { name: es.tab_templates }).click();
	await expect(page).toHaveURL(/\/ajustes\/plantillas$/);
	await tabs.getByRole('link', { name: es.tab_rules }).click();
	await expect(page).toHaveURL(/\/ajustes\/recordatorios$/);
});

test('Enviar has two tabs: Recordatorios and Respuestas', async ({ page }) => {
	await page.goto('/enviar');
	const tabs = page.getByRole('navigation', { name: es.send_tabs_label });
	await expect(page.getByRole('heading', { name: es.send_title, level: 1 })).toBeVisible();
	await expect(tabs.getByRole('link', { name: es.tab_reminders })).toHaveAttribute(
		'aria-current',
		'page'
	);
	await tabs.getByRole('link', { name: es.tab_replies }).click();
	await expect(page).toHaveURL(/\/enviar\/respuestas$/);
	await expect(tabs.getByRole('link', { name: es.tab_replies })).toHaveAttribute(
		'aria-current',
		'page'
	);
});
