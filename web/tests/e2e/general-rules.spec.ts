// General rules, the row label and the handover question (reminders-and-replies design, CUJ R1, R4
// and R6; item 37). Each test adds its own template, service and rules on top of the setup data
// and removes them after, so the other specs see the setup rules only.
import { expect, test, type Page } from '@playwright/test';
import es from '../../src/lib/i18n/es.json' with { type: 'json' };
import {
	at,
	closePool,
	deleteSpecRows,
	handoverOf,
	insertAppointment,
	insertClient,
	insertComeBackRule,
	insertService,
	insertTemplate,
	resetData
} from './seed';
import { dueItem, fill, serverOff } from './steps';

// As in offline.spec.ts: with the service worker blocked, routing sees every request.
test.use({ serviceWorkers: 'block' });

const added = { rules: [] as string[], templates: [] as string[], services: [] as string[] };
let anaId = '';

test.beforeEach(async () => {
	await resetData();
	anaId = await insertClient({ name: 'Ana Rojas', phone: '+50688887777' });
});

test.afterEach(async () => {
	await deleteSpecRows(added);
	added.rules = [];
	added.templates = [];
	added.services = [];
});

test.afterAll(closePool);

/** A service, its template and its rule for one service, plus a general rule on another template. */
async function specificAndGeneral(): Promise<{ rule: string; template: string; service: string }> {
	const service = await insertService('Laminado');
	const template = await insertTemplate('Laminado otra vez', 'Hola {{client.first_name}}');
	const general = await insertTemplate('Te extrañamos', 'Hola {{client.first_name}}, ¿vuelves?');
	const rule = await insertComeBackRule({
		serviceId: service,
		offsetDays: 21,
		templateId: template
	});
	added.services.push(service);
	added.templates.push(template, general);
	added.rules.push(rule);
	await insertComeBackRule({ serviceId: null, offsetDays: 30, templateId: general });
	return { rule, template, service };
}

/** The handover question in the open dialog: answers "Solo de hoy en adelante" or "Incluir
 *  atrasados". */
async function answerHandover(page: Page, include: boolean): Promise<void> {
	const sheet = page.getByRole('dialog', { name: es.overdue_title });
	await expect(sheet).toBeVisible();
	await sheet
		.getByRole('button', { name: include ? es.overdue_include : es.overdue_from_today })
		.click();
	await expect(sheet).toBeHidden();
}

test('CUJ R1: a general rule made with Todos los servicios shows its label', async ({ page }) => {
	const template = await insertTemplate('Te extrañamos', 'Hola {{client.first_name}}, ¿vuelves?');
	added.templates.push(template);
	// Pedicure has no rule of its own, so the general rule takes the visit.
	const visit = await insertAppointment({
		clientId: anaId,
		services: ['Pedicure'],
		startsAt: at(-30, '08:00'),
		createdAt: at(-31, '09:00')
	});

	await page.goto('/ajustes/recordatorios');
	await page.getByRole('button', { name: es.rules_new }).click();
	const sheet = page.getByRole('dialog');
	await sheet
		.getByLabel(es.rule_kind, { exact: true })
		.selectOption({ label: es.rule_kind_come_back });
	await sheet
		.getByLabel(es.rule_template, { exact: true })
		.selectOption({ label: 'Te extrañamos' });
	await sheet.getByLabel(es.rule_days_after, { exact: true }).fill('30');
	await sheet
		.getByLabel(es.rule_service, { exact: true })
		.selectOption({ label: es.rule_service_all });
	// A general rule has no service to book.
	await expect(sheet.getByLabel(es.rule_return, { exact: true })).toHaveCount(0);
	await sheet.getByRole('button', { name: es.rule_save }).click();
	await answerHandover(page, true);
	await expect(
		page.getByText(
			fill(es.rule_sentence_general, { template: 'Te extrañamos', offset: '30 días después' }),
			{ exact: true }
		)
	).toBeVisible();

	await page.goto('/enviar');
	const label = dueItem(page, 'Ana Rojas').getByRole('link', { name: /General 30 días/ });
	await expect(label).toContainText('Pedicure');
	await expect(label).toContainText(/quedan \d+/);
	await expect(label).toHaveAttribute('href', `/citas/${visit}`);
});

test('CUJ R6: offline the label is plain text with no days left', async ({ page }) => {
	await specificAndGeneral();
	await insertAppointment({
		clientId: anaId,
		services: ['Pedicure'],
		startsAt: at(-30, '08:00'),
		createdAt: at(-31, '09:00')
	});
	await page.goto('/enviar');
	await expect(
		dueItem(page, 'Ana Rojas').getByRole('link', { name: /General 30 días/ })
	).toBeVisible();

	await serverOff(page);
	await page.reload();
	const card = dueItem(page, 'Ana Rojas');
	await expect(card.getByText(/General 30 días/)).toBeVisible();
	await expect(card.getByRole('link', { name: /General 30 días/ })).toHaveCount(0);
	await expect(card.getByText(/quedan|queda 1 día|Último día/)).toHaveCount(0);
});

test('CUJ R4: archiving a rule a general rule covers asks the handover question', async ({
	page
}) => {
	const { rule } = await specificAndGeneral();
	await page.goto('/ajustes/recordatorios');
	await page.getByRole('button', { name: /Laminado otra vez/ }).click();
	await page.getByRole('dialog').getByRole('button', { name: es.rule_archive }).click();
	await answerHandover(page, false);
	await expect(page.getByText(es.rule_archived)).toBeVisible();
	expect(await handoverOf(rule)).not.toBeNull();
});

test('archiving its template asks the handover question', async ({ page }) => {
	const { rule, template } = await specificAndGeneral();
	await page.goto(`/ajustes/plantillas/${template}`);
	await page.getByRole('button', { name: es.template_archive }).click();
	await page
		.getByRole('dialog', { name: es.template_archive })
		.getByRole('button', { name: es.template_archive })
		.click();
	await answerHandover(page, true);
	await expect(page.getByText(es.template_archived)).toBeVisible();
	expect(await handoverOf(rule)).toBeNull();
});

test('archiving and restoring its service ask the handover question', async ({ page }) => {
	const { rule } = await specificAndGeneral();
	await page.goto('/servicios/catalogo');
	await page.getByRole('button', { name: /^Laminado/ }).click();
	await page.getByRole('dialog').getByRole('button', { name: es.service_archive }).click();
	await answerHandover(page, false);
	await expect(page.getByText(es.service_archived)).toBeVisible();
	expect(await handoverOf(rule)).not.toBeNull();

	await page.getByRole('button', { name: /^Laminado/ }).click();
	await page.getByRole('dialog').getByRole('button', { name: es.service_restore }).click();
	await answerHandover(page, true);
	await expect(page.getByText(es.service_restored)).toBeVisible();
	expect(await handoverOf(rule)).toBeNull();
});
