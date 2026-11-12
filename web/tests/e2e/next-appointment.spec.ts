// "Agendar próxima cita" (docs/designs/next-appointment.md, CUJ N1 to N4): the button on an
// appointment opens a sheet of date choices from the come-back rules, and the picked one opens
// Nueva cita filled. It builds on the setup data: the services Acrílico, Relleno and Pedicure and
// the rule "21 días después de Acrílico → Relleno". A test that needs another rule adds it and
// removes it after; a rule added here has no return service, so it returns to its own service.
import { expect, test, type Locator, type Page } from '@playwright/test';
import es from '../../src/lib/i18n/es.json' with { type: 'json' };
import {
	at,
	businessDate,
	insertAppointment,
	insertClient,
	insertComeBackRule,
	insertTemplate,
	serviceIdOf,
	ruleSpec
} from './seed';
import { draftStored, dueItem, failDraftWrite, fill } from './steps';

// As in offline.spec.ts: with the service worker blocked, every page load reads the server.
test.use({ serviceWorkers: 'block' });

const { added, ana } = ruleSpec(test);

/** A come-back rule of `days` for the setup service `service`, or a general one for null. */
async function addRule(service: string | null, days: number): Promise<void> {
	const template = await insertTemplate(`Vuelve en ${String(days)}`, 'Hola {{client.first_name}}');
	added.templates.push(template);
	added.rules.push(
		await insertComeBackRule({
			serviceId: service === null ? null : await serviceIdOf(service),
			offsetDays: days,
			templateId: template
		})
	);
}

/** Ana's appointment with `services`, `dayOffset` days from today at `time`. */
const anaVisit = (services: string[], dayOffset: number, time: string): Promise<string> =>
	insertAppointment({ clientId: ana(), services, startsAt: at(dayOffset, time) });

const sheet = (page: Page): Locator => page.getByRole('dialog', { name: es.appointment_next });
/** The dropdown of date choices in the sheet. */
const dateSelect = (page: Page): Locator =>
	sheet(page).getByLabel(es.appointment_day, { exact: true });
/** Its options that are date choices ("En 21 días · 3 dic"), not "Otra fecha". */
const choices = (page: Page): Locator =>
	dateSelect(page).getByRole('option', { name: /^En \d+ días? · / });
/** The line under the dropdown: the services the selected date books. */
const servicesLine = (page: Page, names: string): Locator =>
	sheet(page).getByText(`${es.appointment_services}: ${names}`, { exact: true });
const useDate = (page: Page): Locator =>
	sheet(page).getByRole('button', { name: es.next_use_date, exact: true });
/** `3 dic`: a day as the app's `shortDay` writes it. */
function shortDay(day: string): string {
	const month = es[`month_${String(Number(day.slice(5, 7)))}` as keyof typeof es];
	return `${String(Number(day.slice(8)))} ${month.slice(0, 3)}`;
}
/** Selects the choice of `days` in the dropdown; `day` is the date its text names. */
async function selectChoice(page: Page, days: number, day: string): Promise<void> {
	await dateSelect(page).selectOption({
		label: fill(es.next_choice_other, { n: String(days), date: shortDay(day) })
	});
}
const removeButton = (page: Page, name: string): Locator =>
	page.getByRole('button', { name: fill(es.appointment_service_remove, { name }) });

/** Ana's client page → her appointment at `time` → "Agendar próxima cita". Reached inside the
 *  app, so the appointment's back arrow goes back in the history. */
async function openSheet(page: Page, time: string): Promise<void> {
	await page.goto(`/clientas/${ana()}`);
	await page.getByRole('link', { name: new RegExp(` · ${time}`) }).click();
	await expect(page.getByRole('heading', { name: es.appointment_title, level: 1 })).toBeVisible();
	await page.getByRole('button', { name: es.appointment_next, exact: true }).click();
	await expect(sheet(page)).toBeVisible();
}

/** Nueva cita holds this client, these services and no others of `absent`, on `day` at `time`. */
async function expectFilled(
	page: Page,
	want: { client: string; services: string[]; absent: string[]; day: string; time: string }
): Promise<void> {
	await expect(page.getByRole('heading', { name: es.appointment_new, level: 1 })).toBeVisible();
	await expect(page.getByText(want.client)).toBeVisible();
	for (const name of want.services) {
		await expect(removeButton(page, name)).toBeVisible();
	}
	for (const name of want.absent) {
		await expect(removeButton(page, name)).toHaveCount(0);
	}
	await expect(page.getByLabel(es.appointment_day, { exact: true })).toHaveValue(want.day);
	await expect(page.getByLabel(es.appointment_time, { exact: true })).toHaveValue(want.time);
}

test('CUJ N1: the rule names the next service, and its reminder goes', async ({ page }) => {
	await addRule('Pedicure', 30);
	const visit = await insertAppointment({
		clientId: ana(),
		services: ['Acrílico', 'Pedicure'],
		startsAt: at(-21, '14:30'),
		createdAt: at(-22, '09:00')
	});
	// Marta's come-back is due too: once her item shows, the list on screen is the server's.
	await insertAppointment({
		clientId: await insertClient({ name: 'Marta Solano', phone: '+50688886666' }),
		services: ['Acrílico'],
		startsAt: at(-21, '09:00'),
		createdAt: at(-22, '09:00')
	});
	/** Ana's come-back item: the one with the text of the setup template "Relleno". */
	const comeBack = dueItem(page, 'Ana Rojas').filter({ hasText: 'ya toca tu relleno' });
	await page.goto('/enviar');
	await expect(comeBack).toBeVisible();

	await openSheet(page, '14:30');
	// The first choice is selected when the sheet opens.
	await expect(choices(page)).toHaveCount(2);
	await expect(servicesLine(page, 'Relleno, Pedicure')).toBeVisible();
	await selectChoice(page, 30, businessDate(9));
	await expect(servicesLine(page, 'Relleno, Pedicure')).toBeVisible();
	await selectChoice(page, 21, businessDate(0));
	await useDate(page).click();

	await expect(page).toHaveURL(new RegExp(`/citas/nueva\\?volver=/citas/${visit}$`));
	await expectFilled(page, {
		client: 'Ana Rojas',
		services: ['Relleno', 'Pedicure'],
		absent: ['Acrílico'],
		day: businessDate(0),
		time: '14:30'
	});
	await expect(page.getByLabel(es.appointment_notes, { exact: true })).toHaveValue('');

	await page.getByRole('button', { name: es.appointment_save }).click();
	// Save returns to the visit she started from and says what was booked: that page does not
	// show the new appointment. Its back arrow goes where it went before.
	await expect(page).toHaveURL(new RegExp(`/citas/${visit}$`));
	await expect(
		page.getByText(fill(es.appointment_created, { date: shortDay(businessDate(0)) }), {
			exact: true
		})
	).toBeVisible();
	await page.getByRole('button', { name: es.appointment_back }).click();
	await expect(page.getByRole('heading', { name: 'Ana Rojas', level: 1 })).toBeVisible();

	await page.goto('/enviar');
	await expect(dueItem(page, 'Marta Solano')).toBeVisible();
	await expect(comeBack).toHaveCount(0);
});

test('CUJ N2: only the general rule', async ({ page }) => {
	await addRule(null, 30);
	await anaVisit(['Pedicure'], 1, '10:00');

	await openSheet(page, '10:00');
	await expect(choices(page)).toHaveCount(1);
	await expect(servicesLine(page, 'Pedicure')).toBeVisible();
	await useDate(page).click();

	await expectFilled(page, {
		client: 'Ana Rojas',
		services: ['Pedicure'],
		absent: ['Acrílico', 'Relleno'],
		day: businessDate(31),
		time: '10:00'
	});
});

test('CUJ N3: no rule, another date', async ({ page }) => {
	const visit = await anaVisit(['Pedicure'], 1, '10:00');

	await openSheet(page, '10:00');
	// No rule gives a date, so there is no dropdown: only the date field.
	await expect(dateSelect(page)).toHaveCount(0);
	await expect(servicesLine(page, 'Pedicure')).toBeVisible();
	const field = sheet(page).getByLabel(es.next_other_date, { exact: true });
	await expect(useDate(page)).toBeDisabled();
	await expect(useDate(page)).toHaveAccessibleDescription(es.appointment_day_needed);
	await field.fill(businessDate(10));
	await expect(useDate(page)).toBeEnabled();
	// Emptied again, the button is off again.
	await field.fill('');
	await expect(useDate(page)).toBeDisabled();
	await field.fill(businessDate(10));
	await useDate(page).click();

	await expectFilled(page, {
		client: 'Ana Rojas',
		services: ['Pedicure'],
		absent: ['Acrílico', 'Relleno'],
		day: businessDate(10),
		time: '10:00'
	});

	// The header arrow goes back to the appointment and leaves no Nueva cita behind in the history.
	await page.getByRole('link', { name: es.appointment_back }).click();
	// The URL, not the heading: "Cita" is also inside "Nueva cita".
	await expect(page).toHaveURL(new RegExp(`/citas/${visit}$`));
	await page.goBack();
	await expect(page.getByRole('heading', { name: 'Ana Rojas', level: 1 })).toBeVisible();
	await expect(page.getByRole('heading', { name: es.appointment_new, level: 1 })).toHaveCount(0);
});

test('CUJ N3b: the services follow the picked days', async ({ page }) => {
	await addRule('Acrílico', 60);
	await anaVisit(['Acrílico'], 1, '09:00');

	await openSheet(page, '09:00');
	await expect(servicesLine(page, 'Relleno')).toBeVisible();
	await selectChoice(page, 60, businessDate(61));
	await expect(servicesLine(page, 'Acrílico')).toBeVisible();
	await useDate(page).click();

	await expectFilled(page, {
		client: 'Ana Rojas',
		services: ['Acrílico'],
		absent: ['Relleno', 'Pedicure'],
		day: businessDate(61),
		time: '09:00'
	});
});

test.describe('with an unsaved Nueva cita', () => {
	test.beforeEach(async ({ page }) => {
		// Three choices: 21, 30 and 60 days.
		await addRule('Pedicure', 30);
		await addRule('Acrílico', 60);
		await insertClient({ name: 'Marta Solano', phone: '+50688886666' });
		await anaVisit(['Acrílico', 'Pedicure'], 1, '09:00');
		await page.goto('/citas/nueva');
		await page.getByLabel(es.appointment_client, { exact: true }).fill('marta');
		await page.getByRole('button', { name: 'Marta Solano', exact: true }).click();
		await page.getByRole('button', { name: 'Acrílico', exact: true }).click();
		await expect.poll(() => draftStored(page, 'cita')).toBe(true);
		await openSheet(page, '09:00');
		await useDate(page).click();
		await expect(sheet(page).getByText(es.draft_replace_prompt)).toBeVisible();
	});

	test('CUJ N4: Cancelar keeps the unsaved booking', async ({ page }) => {
		// One "Cancelar" on screen: the question's. The sheet's own is hidden while it asks.
		const cancel = sheet(page).getByRole('button', { name: es.cancel, exact: true });
		await expect(cancel).toHaveCount(1);
		await cancel.click();

		await expect(sheet(page).getByText(es.draft_replace_prompt)).toHaveCount(0);
		await expect(sheet(page)).toBeVisible();
		await page.goto('/citas/nueva');
		await expect(page.getByText('Marta Solano')).toBeVisible();
		await expect(removeButton(page, 'Acrílico')).toBeVisible();
		await expect(removeButton(page, 'Relleno')).toHaveCount(0);
	});

	test('the question is in view on the phone', async ({ page }) => {
		await expect(
			sheet(page).getByRole('button', { name: es.draft_replace, exact: true })
		).toBeInViewport({ ratio: 1 });
	});

	test('Reemplazar books the date selected now', async ({ page }) => {
		// Another date while the question shows: the question goes, and comes back for that date.
		await selectChoice(page, 60, businessDate(61));
		await expect(sheet(page).getByText(es.draft_replace_prompt)).toHaveCount(0);
		await useDate(page).click();
		await expect(sheet(page).getByText(es.draft_replace_prompt)).toBeVisible();
		await sheet(page).getByRole('button', { name: es.draft_replace, exact: true }).click();

		await expectFilled(page, {
			client: 'Ana Rojas',
			services: ['Acrílico', 'Pedicure'],
			absent: ['Relleno'],
			day: businessDate(61),
			time: '09:00'
		});
	});

	test('CUJ N4: Reemplazar puts the picked choice into Nueva cita', async ({ page }) => {
		await sheet(page).getByRole('button', { name: es.draft_replace, exact: true }).click();

		await expectFilled(page, {
			client: 'Ana Rojas',
			services: ['Relleno', 'Pedicure'],
			absent: ['Acrílico'],
			day: businessDate(22),
			time: '09:00'
		});
		await expect(page.getByText('Marta Solano')).toHaveCount(0);
	});
});

test('a failed write says so and the sheet stays', async ({ page }) => {
	const logs: string[] = [];
	page.on('console', (message) => {
		if (message.type() === 'warning') {
			logs.push(message.text());
		}
	});
	await failDraftWrite(page, 'cita:new');
	const visit = await anaVisit(['Acrílico'], 1, '09:00');

	await openSheet(page, '09:00');
	await useDate(page).click();

	await expect(
		sheet(page).getByRole('alert').filter({ hasText: es.cita_handoff_failed })
	).toBeVisible();
	await expect(page).toHaveURL(new RegExp(`/citas/${visit}$`));
	expect(logs).toContain('code=cita_handoff_failed');
	expect(logs.join('\n')).not.toMatch(/Ana|Relleno|Acrílico/);
});

test('a cancelled appointment has no button', async ({ page }) => {
	const cancelled = await insertAppointment({
		clientId: ana(),
		services: ['Acrílico'],
		startsAt: at(1, '09:00'),
		status: 'cancelled'
	});

	await page.goto(`/citas/${cancelled}`);

	await expect(
		page.getByRole('button', { name: es.appointment_reactivate, exact: true })
	).toBeVisible();
	await expect(page.getByRole('button', { name: es.appointment_next, exact: true })).toHaveCount(0);
});
