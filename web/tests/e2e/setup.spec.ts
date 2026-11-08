// The `setup` project (design doc item 79): CUJ 1 (handover and first setup, without the real
// install and push) and CUJ 2 (she configures the empty app), in the real wizard and screens. It
// saves the logged-in state that every other spec reuses, and the data it creates (plan D4) is
// what those specs build on.
import { execFileSync } from 'node:child_process';
import { tmpdir } from 'node:os';
import { expect, test, type Page } from '@playwright/test';
import es from '../../src/lib/i18n/es.json' with { type: 'json' };
import { AUTH_FILE, OWNER, ZONE, appEnv } from './env';
import { fill } from './steps';

const BUSINESS = 'Estudio Prueba';
/** The built-in preview client (internal/locale/es.go); it lives in Go, so the spec types it. */
const SAMPLE_CLIENT = 'Ana Pérez';

const SERVICES = [
	{ name: 'Acrílico', minutes: '90', price: '45.00' },
	{ name: 'Relleno', minutes: '60', price: '30.00' },
	{ name: 'Pedicure', minutes: '60', price: '25.00' }
];
const TEMPLATES = [
	{ name: 'Recordatorio de cita', text: 'te esperamos mañana. ' },
	{ name: 'Relleno', text: 'ya toca tu relleno. ' }
];

/** Ajustes → one section, through the tab bar and the section list. */
async function openSettings(page: Page, section: string): Promise<void> {
	await page
		.getByRole('navigation', { name: es.nav_label })
		.getByRole('link', { name: es.nav_settings })
		.click();
	await page
		.getByRole('navigation', { name: es.settings_sections })
		.getByRole('link', { name: section })
		.click();
}

test.describe.configure({ mode: 'serial' });

// The link `setup-link` prints; the first test opens it, the last one checks it is used up.
let link = '';

test('the setup link opens the wizard and setup creates her account', async ({ page }) => {
	// setup-link reads the database only; its MEDIA_DIR is never used.
	link = execFileSync('../bin/open-customer', ['setup-link'], {
		env: appEnv(tmpdir()),
		encoding: 'utf8'
	}).trim();
	await page.goto(link);
	await expect(page.getByRole('heading', { name: es.setup_title })).toBeVisible();
	const next = page.getByRole('button', { name: es.form_next });

	await page.getByLabel(es.setup_business_label, { exact: true }).fill(BUSINESS);
	await next.click();
	await page.getByLabel(es.setup_timezone_label, { exact: true }).selectOption(ZONE);
	await next.click();
	await page.getByLabel(es.setup_currency_label, { exact: true }).selectOption('USD');
	await next.click();
	await page.getByLabel(es.setup_country_label, { exact: true }).selectOption('CR');
	await next.click();
	// The default color stays.
	await expect(page.getByRole('heading', { name: es.setup_color_title })).toBeVisible();
	await next.click();
	await page.getByLabel(es.setup_username_label, { exact: true }).fill(OWNER.username);
	await next.click();
	await page.getByLabel(es.setup_password_label, { exact: true }).fill(OWNER.password);
	await page.getByRole('button', { name: es.setup_create }).click();

	await expect(page.getByRole('heading', { name: es.setup_icon_title })).toBeVisible();
	// A reload after the account exists keeps her on the wizard's last steps.
	await page.reload();
	await expect(page.getByRole('heading', { name: es.setup_icon_title })).toBeVisible();
	await page.getByRole('button', { name: es.setup_icon_skip }).click();
	await expect(page.getByRole('heading', { name: es.install_title })).toBeVisible();
	await page.getByRole('button', { name: es.setup_enter }).click();
	await expect(page.getByRole('heading', { name: es.today_title, level: 1 })).toBeVisible();
});

test('she logs in on a new device and the app carries her business name', async ({ page }) => {
	// A new browser context: the installed app has no session of its own yet.
	await page.goto('/');
	await expect(page.getByRole('heading', { name: es.login_heading })).toBeVisible();
	await page.getByLabel(es.login_username, { exact: true }).fill(OWNER.username);
	await page.getByLabel(es.login_password, { exact: true }).fill(OWNER.password);
	await page.getByRole('button', { name: es.login_submit }).click();
	await expect(page.getByRole('heading', { name: es.today_title, level: 1 })).toBeVisible();

	// The installed app's name comes from the manifest.
	const manifest = await page.request.get('/manifest.webmanifest');
	const body: unknown = await manifest.json();
	expect(body).toMatchObject({ name: BUSINESS });

	await page.context().storageState({ path: AUTH_FILE });
});

test.describe('logged in', () => {
	test.use({ storageState: AUTH_FILE });

	test('she configures services, attributes, templates and rules in the empty app', async ({
		page
	}) => {
		await page.goto('/');
		// No rules yet: Hoy leads to Ajustes → Recordatorios.
		await expect(page.getByRole('link', { name: es.due_no_rules_action })).toBeVisible();

		// CUJ 2 step 1: services.
		await page
			.getByRole('navigation', { name: es.nav_label })
			.getByRole('link', { name: es.nav_services, exact: true })
			.click();
		await expect(page).toHaveURL(/\/servicios\/catalogo$/);
		await expect(page.getByText(es.appointment_services_empty_title)).toBeVisible();
		for (const service of SERVICES) {
			await page.getByRole('button', { name: es.services_new }).click();
			const sheet = page.getByRole('dialog', { name: es.service_new_title });
			await sheet.getByLabel(es.service_name, { exact: true }).fill(service.name);
			await sheet.getByLabel(es.service_duration, { exact: true }).fill(service.minutes);
			await sheet
				.getByLabel(fill(es.service_price, { currency: 'USD' }), { exact: true })
				.fill(service.price);
			await sheet.getByRole('button', { name: es.service_save }).click();
			await expect(sheet).toBeHidden();
			await expect(page.getByRole('main').getByText(service.name, { exact: true })).toBeVisible();
		}

		// CUJ 2 step 2: a client attribute (text) and an appointment attribute (select).
		await openSettings(page, es.settings_attributes);
		await expect(page.getByText(es.attributes_empty_title)).toBeVisible();
		await page.getByRole('button', { name: es.attributes_new }).click();
		let sheet = page.getByRole('dialog', { name: es.attribute_new_title });
		await sheet.getByLabel(es.attribute_label, { exact: true }).fill('Bebida favorita');
		await sheet.getByLabel(es.attribute_scope_client, { exact: true }).check();
		await sheet.getByLabel(es.attribute_type, { exact: true }).selectOption({
			label: es.attribute_type_text
		});
		await sheet.getByRole('button', { name: es.attribute_create }).click();
		await expect(sheet).toBeHidden();
		await expect(
			page.getByRole('main').getByText('Bebida favorita', { exact: true })
		).toBeVisible();

		await page.getByRole('button', { name: es.attributes_new }).click();
		sheet = page.getByRole('dialog', { name: es.attribute_new_title });
		await sheet.getByLabel(es.attribute_label, { exact: true }).fill('Forma');
		await sheet.getByLabel(es.attribute_scope_appointment, { exact: true }).check();
		await sheet.getByLabel(es.attribute_type, { exact: true }).selectOption({
			label: es.attribute_type_select
		});
		await sheet.getByRole('button', { name: es.attribute_create }).click();
		// A select goes on to its options, in the same attribute's sheet.
		sheet = page.getByRole('dialog', { name: 'Forma' });
		for (const option of ['Almendra', 'Cuadrada']) {
			await sheet.getByRole('button', { name: es.option_add }).click();
			await sheet.getByLabel(es.option_label, { exact: true }).fill(option);
			await sheet.getByRole('button', { name: es.option_add_confirm }).click();
			await expect(
				sheet.getByRole('button', { name: fill(es.option_menu, { label: option }) })
			).toBeVisible();
		}
		await sheet.getByRole('button', { name: es.cancel }).click();
		await expect(sheet).toBeHidden();

		// CUJ 2 step 3: two messages, each with the "+ Bebida favorita" chip and a live preview.
		await openSettings(page, es.settings_templates);
		await expect(page.getByText(es.templates_empty_title)).toBeVisible();
		for (const template of TEMPLATES) {
			await page.getByRole('link', { name: es.templates_new }).click();
			await page.getByLabel(es.template_name, { exact: true }).fill(template.name);
			await page.getByRole('textbox', { name: es.template_message }).click();
			await page.keyboard.type('Hola ');
			await page.getByRole('button', { name: '+ Nombre', exact: true }).click();
			await page.keyboard.type(`, ${template.text}`);
			await page.getByRole('button', { name: '+ Bebida favorita', exact: true }).click();
			const preview = page.getByRole('region', { name: es.preview_with }).getByRole('status');
			await expect(preview).toContainText(`Hola ${SAMPLE_CLIENT}, ${template.text}`);
			await expect(preview).toContainText('[Bebida favorita]');
			await page.getByRole('button', { name: es.template_save }).click();
			await expect(page.getByRole('heading', { name: template.name, level: 2 })).toBeVisible();
			await page.getByRole('link', { name: es.template_back }).click();
		}

		// CUJ 2 steps 3 and 4: the "before" rule and the come-back rule on Acrílico.
		await openSettings(page, es.settings_rules);
		await expect(page.getByText(es.due_no_rules_title)).toBeVisible();
		await page.getByRole('button', { name: es.rules_new }).click();
		sheet = page.getByRole('dialog');
		await sheet.getByLabel(es.rule_kind, { exact: true }).selectOption({
			label: es.rule_kind_before
		});
		await sheet.getByLabel(es.rule_template, { exact: true }).selectOption({
			label: 'Recordatorio de cita'
		});
		await sheet.getByLabel(es.rule_days_before, { exact: true }).fill('1');
		const created = page.waitForRequest(
			(r) => r.method() === 'POST' && r.url().endsWith('/api/v1/reminder-rules')
		);
		await sheet.getByRole('button', { name: es.rule_save }).click();
		// A "before" rule never brings late messages: it saves without the overdue question, and
		// includes the visits still ahead whose message day just passed.
		expect((await created).postDataJSON()).toMatchObject({
			kind: 'before_appointment',
			include_overdue: true
		});
		await expect(sheet).toBeHidden();
		await expect(page.getByRole('heading', { name: es.overdue_title })).toHaveCount(0);
		await expect(
			page.getByText(
				fill(es.rule_sentence_before, {
					template: 'Recordatorio de cita',
					offset: es.offset_before_one
				}),
				{ exact: true }
			)
		).toBeVisible();

		await page.getByRole('button', { name: es.rules_new }).click();
		await sheet.getByLabel(es.rule_kind, { exact: true }).selectOption({
			label: es.rule_kind_come_back
		});
		await sheet.getByLabel(es.rule_template, { exact: true }).selectOption({ label: 'Relleno' });
		await sheet.getByLabel(es.rule_days_after, { exact: true }).fill('21');
		await sheet.getByLabel(es.rule_service, { exact: true }).selectOption({ label: 'Acrílico' });
		await sheet.getByLabel(es.rule_return, { exact: true }).selectOption({ label: 'Relleno' });
		await sheet.getByRole('button', { name: es.rule_save }).click();
		await expect(sheet.getByRole('heading', { name: es.overdue_title })).toBeVisible();
		await sheet.getByRole('button', { name: es.overdue_from_today }).click();
		await expect(sheet).toBeHidden();
		await expect(
			page.getByText(
				fill(es.rule_sentence_come_back, {
					template: 'Relleno',
					offset: fill(es.offset_after_other, { n: '21' }),
					service: 'Acrílico',
					returnService: 'Relleno'
				}),
				{ exact: true }
			)
		).toBeVisible();
	});
});

test('the setup link does not work after setup', async ({ page }) => {
	await page.goto(link);
	// Setup is done and this device has no session: the app sends her to the login screen.
	await expect(page.getByRole('heading', { name: es.login_heading })).toBeVisible();
	await expect(page.getByLabel(es.setup_business_label, { exact: true })).toHaveCount(0);
});

test('a wizard step left in the tab is cleared when /setup sends her away', async ({ page }) => {
	// A tab that once held the finishing wizard, now without a session: /setup sends her to log
	// in, and the stored step must go, so a later sign-in does not reopen the wizard.
	await page.goto('/login');
	await page.evaluate(() => {
		sessionStorage.setItem('open-customer.setup-finishing', 'install');
	});
	await page.goto('/setup');
	await expect(page.getByRole('heading', { name: es.login_heading })).toBeVisible();

	expect(
		await page.evaluate(() => sessionStorage.getItem('open-customer.setup-finishing'))
	).toBeNull();
});
