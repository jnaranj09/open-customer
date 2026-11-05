// CUJ 8 (design doc item 79): Ajustes → "Descargar mis datos" downloads the export ZIP. The file
// names and columns are the design doc's "Export" section; the services, attributes, templates and
// rules are the ones `setup.spec.ts` creates in CUJ 2.
import { readFile } from 'node:fs/promises';
import { expect, test } from '@playwright/test';
import es from '../../src/lib/i18n/es.json' with { type: 'json' };
import { at, closePool, insertAppointment, insertClient, resetData } from './seed';
import { readZip } from './zip';

test.beforeEach(async () => {
	await resetData();
	const ana = await insertClient({
		name: 'Ana Rojas',
		phone: '+50688887777',
		attrs: { 'Bebida favorita': 'Café' }
	});
	await insertAppointment({ clientId: ana, services: ['Acrílico'], startsAt: at(1, '15:00') });
});

test.afterAll(closePool);

/** The UTF-8 BOM the CSV files start with. */
const BOM = String.fromCodePoint(0xfeff);

/** The rows of a CSV file without its BOM; the seeded values hold no comma or quote. */
function csvRows(text: string): string[][] {
	return text
		.replace(BOM, '')
		.trimEnd()
		.split(/\r?\n/)
		.map((line) => line.split(','));
}

/** The values of `column` in a CSV file, one per data row. */
function csvColumn(text: string, column: string): string[] {
	const [header = [], ...rows] = csvRows(text);
	const index = header.indexOf(column);
	expect(index, `column ${column}`).toBeGreaterThanOrEqual(0);
	return rows.map((row) => row[index] ?? '');
}

test('Descargar mis datos downloads the data as CSV and JSON', async ({ page }) => {
	await page.goto('/ajustes');
	await page.getByRole('link', { name: es.settings_export, exact: true }).click();
	const download = page.waitForEvent('download');
	await page.getByRole('link', { name: es.export_button, exact: true }).click();
	const files = readZip(await readFile(await (await download).path()));

	expect([...files.keys()]).toEqual(['clientas.csv', 'citas.csv', 'servicios.csv', 'datos.json']);
	const clients = files.get('clientas.csv') ?? '';
	expect(csvColumn(clients, 'nombre')).toEqual(['Ana Rojas']);
	expect(csvColumn(clients, 'Bebida favorita')).toEqual(['Café']);
	expect(csvColumn(files.get('citas.csv') ?? '', 'clienta')).toEqual(['Ana Rojas']);
	expect(csvColumn(files.get('servicios.csv') ?? '', 'nombre')).toEqual([
		'Acrílico',
		'Relleno',
		'Pedicure'
	]);

	const data = JSON.parse(files.get('datos.json') ?? '{}') as {
		attributes: { label: string }[];
		templates: { name: string }[];
		rules: { kind: string }[];
		clients: { name: string }[];
		appointments: { client_name: string }[];
	};
	expect(data.attributes.map((a) => a.label)).toEqual(
		expect.arrayContaining(['Bebida favorita', 'Forma'])
	);
	expect(data.templates.map((t) => t.name)).toEqual(
		expect.arrayContaining(['Recordatorio de cita', 'Relleno'])
	);
	expect(data.rules.map((r) => r.kind).sort()).toEqual(['before_appointment', 'come_back']);
	expect(data.clients.map((c) => c.name)).toEqual(['Ana Rojas']);
	expect(data.appointments.map((a) => a.client_name)).toEqual(['Ana Rojas']);
});
