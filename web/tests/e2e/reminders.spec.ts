// CUJ 4 (morning sending) and CUJ 5 (the come-back reminder lifecycle), design doc item 79. They
// use the rules and templates `setup.spec.ts` creates in CUJ 2 (plan D4, D5): the 1-day "before"
// rule with "Recordatorio de cita", and the 21-day come-back rule on Acrílico with "Relleno" and
// the return service Relleno. This spec creates no rule of its own.
import { expect, test, type Page } from '@playwright/test';
import es from '../../src/lib/i18n/es.json' with { type: 'json' };
import { at, businessDate, closePool, insertAppointment, insertClient, resetData } from './seed';
import { dueItem, fill } from './steps';

const WA_ME = 'https://wa.me/';

let anaId = '';

test.beforeEach(async () => {
	await resetData();
	anaId = await insertClient({
		name: 'Ana Rojas',
		phone: '+50688887777',
		attrs: { 'Bebida favorita': 'Café' }
	});
});

test.afterAll(closePool);

test.describe('CUJ 4: morning sending', () => {
	test.beforeEach(async ({ page }) => {
		const marta = await insertClient({ name: 'Marta Solano', phone: '+50688886666' });
		await insertAppointment({ clientId: anaId, services: ['Acrílico'], startsAt: at(1, '15:00') });
		await insertAppointment({ clientId: marta, services: ['Acrílico'], startsAt: at(1, '10:00') });
		// WhatsApp is outside the test: the navigation lands on a stand-in page.
		await page.route(`${WA_ME}**`, (route) =>
			route.fulfill({ contentType: 'text/html', body: '<!doctype html><title>WhatsApp</title>' })
		);
	});

	/** "Para enviar hoy" → "Enviar" on Ana's item; returns the message the wa.me link carries. */
	async function sendToAna(page: Page): Promise<string> {
		await page.goto('/enviar');
		await expect(page.getByRole('heading', { name: es.due_today_heading, level: 1 })).toBeVisible();
		await dueItem(page, 'Ana Rojas')
			.getByRole('button', { name: es.due_send, exact: true })
			.click();
		await page.waitForURL(`${WA_ME}**`);
		expect(page.url()).toMatch(/^https:\/\/wa\.me\/50688887777\?text=/);
		return new URL(page.url()).searchParams.get('text') ?? '';
	}

	test('Enviar opens WhatsApp with the rendered message', async ({ page }) => {
		const text = await sendToAna(page);
		expect(text).toContain('Ana Rojas');
		expect(text).toContain('Café');
	});

	test('Sí moves the item to Enviados and it stays there after a reload', async ({ page }) => {
		const text = await sendToAna(page);

		// She comes back from WhatsApp: the stored confirmation asks first.
		await page.goto('/enviar');
		const sheet = page.getByRole('dialog', {
			name: fill(es.due_confirm_sent, { name: 'Ana Rojas' })
		});
		await expect(sheet).toBeVisible();
		await sheet.getByRole('button', { name: es.due_confirm_yes }).click();
		await expect(sheet).toBeHidden();

		const expectSent = async (): Promise<void> => {
			// Under "Enviados" a row is a plain line, not a card with a heading and buttons.
			await expect(page.getByRole('heading', { name: es.due_sent_heading })).toBeVisible();
			await expect(page.getByRole('listitem').filter({ hasText: 'Ana Rojas' })).toContainText(text);
			await expect(dueItem(page, 'Ana Rojas')).toHaveCount(0);
			await expect(
				dueItem(page, 'Marta Solano').getByRole('button', { name: es.due_send, exact: true })
			).toBeVisible();
			await expect(page.getByText(es.due_count_one, { exact: true })).toBeVisible();
		};
		await expectSent();
		await page.reload();
		await expectSent();
	});
});

test.describe('CUJ 5: come-back reminder lifecycle', () => {
	/** An Acrílico visit of `clientId` that ended 21 days ago (14:30–16:00), typed in a day before. */
	function visit(clientId: string, status?: 'no_show'): Promise<string> {
		return insertAppointment({
			clientId,
			services: ['Acrílico'],
			startsAt: at(-21, '14:30'),
			createdAt: at(-22, '09:00'),
			...(status === undefined ? {} : { status })
		});
	}

	/** Marta has a due come-back too: once her item shows, the list on screen is the server's. */
	async function martaIsDue(page: Page): Promise<void> {
		await visit(await insertClient({ name: 'Marta Solano', phone: '+50688886666' }));
		await page.goto('/enviar');
		await expect(dueItem(page, 'Marta Solano')).toBeVisible();
	}

	test('the come-back message is due 21 days after the visit', async ({ page }) => {
		await visit(anaId);
		await page.goto('/enviar');

		const item = dueItem(page, 'Ana Rojas');
		await expect(item).toContainText('Café');
		await expect(item.getByRole('button', { name: es.due_send, exact: true })).toBeVisible();
		// Due today, not late: it is under "Hoy", and there is no "Atrasados" group.
		await expect(page.getByRole('heading', { name: es.today_title, level: 2 })).toBeVisible();
		await expect(page.getByRole('heading', { name: es.due_overdue_heading })).toHaveCount(0);
	});

	test('a later Relleno booking hides the message until it is cancelled', async ({ page }) => {
		await visit(anaId);
		const relleno = await insertAppointment({
			clientId: anaId,
			services: ['Relleno'],
			startsAt: at(7, '10:00')
		});
		await martaIsDue(page);
		await expect(dueItem(page, 'Ana Rojas')).toHaveCount(0);

		await page.goto(`/citas/${relleno}`);
		await page.getByRole('button', { name: es.appointment_cancel, exact: true }).click();
		await expect(page.getByText(es.status_cancelled, { exact: true })).toBeVisible();

		await page.goto('/enviar');
		await expect(dueItem(page, 'Ana Rojas')).toContainText('Café');
	});

	test('a later Pedicure booking shows the booking prompt', async ({ page }) => {
		await visit(anaId);
		const pedicure = await insertAppointment({
			clientId: anaId,
			services: ['Pedicure'],
			startsAt: at(7, '10:00')
		});
		// The app shows the day as d/m, without leading zeros.
		const [, month, day] = businessDate(7).split('-').map(Number);
		await page.goto('/enviar');

		const item = dueItem(page, 'Ana Rojas');
		await expect(
			item.getByText(
				fill(es.due_prompt, {
					name: 'Ana Rojas',
					day: `${String(day)}/${String(month)}`,
					services: 'Pedicure',
					service: 'Relleno'
				}),
				{ exact: true }
			)
		).toBeVisible();
		await item.getByRole('button', { name: es.appointment_add_service, exact: true }).click();
		await expect(item).toHaveCount(0);

		await page.goto(`/citas/${pedicure}`);
		await expect(page.getByRole('heading', { name: 'Ana Rojas', level: 2 })).toBeVisible();
		await expect(page.getByRole('listitem').filter({ hasText: 'Relleno' })).toBeVisible();
		await expect(page.getByRole('listitem').filter({ hasText: 'Pedicure' })).toBeVisible();
	});

	test('No vino means the message never appears', async ({ page }) => {
		await visit(anaId, 'no_show');
		await martaIsDue(page);
		await expect(dueItem(page, 'Ana Rojas')).toHaveCount(0);
	});
});
