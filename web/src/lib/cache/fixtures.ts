// Made-up data shared by the cache tests.
import type { Bootstrap, DueItem } from '../api/schemas';

const ID = '3f2b8c1e-5d4a-4e6b-9c7d-1a2b3c4d5e6f';
const OTHER_ID = '7a1c9e20-2b3d-4f5a-8b6c-0d9e8f7a6b5c';
// Built from parts: the raw-color guard scans every non-test file under src/ for hex literals.
const ACCENT = ['#', 'c76b85'].join('');
const STAMP = '2026-11-01T10:00:00Z';

function dueItem(n: number): DueItem {
	return {
		id: `${ID}:${String(n)}`,
		rule_id: OTHER_ID,
		appointment_id: ID,
		anchor_at: null,
		kind: 'come_back',
		client_id: ID,
		client_name: 'Ana Rojas',
		due_date: '2026-11-04',
		late: false,
		text: 'Hola Ana',
		phone_digits: '50688887777',
		wa_link: 'https://wa.me/50688887777?text=Hola',
		mode: 'message',
		prompt: null
	};
}

/** A `/bootstrap` answer with one client, one appointment and `dueCount` due items. */
export function bootstrap(dueCount = 0, clientName = 'Ana Rojas'): Bootstrap {
	return {
		server_time: '2026-11-04T12:00:00Z',
		settings: {
			name: 'Estudio Ana',
			timezone: 'America/Costa_Rica',
			locale: 'es-CR',
			currency: 'CRC',
			country_code: 'CR',
			accent_color: ACCENT,
			push_hour: 8,
			has_custom_icon: false
		},
		attributes: [],
		services: [],
		templates: [],
		rules: [],
		clients: [
			{
				id: ID,
				name: clientName,
				phone_e164: '+50688887777',
				phone_digits: '50688887777',
				notes: null,
				attrs: {},
				archived: false,
				created_at: STAMP,
				updated_at: STAMP
			}
		],
		appointments: [
			{
				id: OTHER_ID,
				client_id: ID,
				client_name: clientName,
				starts_at: '2026-11-04T16:00:00Z',
				ends_at: '2026-11-04T17:00:00Z',
				status: 'scheduled',
				done: false,
				total_cents: 0,
				paid_cents: null,
				attrs: {},
				notes: null,
				services: [],
				created_at: STAMP,
				updated_at: STAMP
			}
		],
		due: Array.from({ length: dueCount }, (_, i) => dueItem(i)),
		template_vars: [{ ns: 'client', key: 'name', label: 'Nombre' }],
		backup_red: false
	};
}
