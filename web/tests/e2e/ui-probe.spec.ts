// UI pipeline stage 8, runtime sensors (.claude/rules/ui.md): every route of the app, at the phone
// profile (`probe-compact`, 393 x 852) and the desktop size (`probe-wide`, 1440 x 900), through axe
// (WCAG 2.0 to 2.2 A/AA) and the layout probe (sideways scroll, 44 px tap targets, 16 px field font,
// cut text), with a 60-letter one-word client name seeded. Every opened sheet gets the same checks
// and the page behind it must be inert. Every data feed of a screen is answered empty, with a 500
// and not at all, and the screen must show its empty text, an alert and a busy status. A route on
// disk with no entry here fails. Known findings are in `ui-probe.allow.ts`.
import { readdirSync } from 'node:fs';
import { join, relative, sep } from 'node:path';
import { expect, test, type Locator, type Page, type Request, type Route } from '@playwright/test';
import es from '../../src/lib/i18n/es.json' with { type: 'json' };
import { AUTH_FILE } from './env';
import { axe, behindSheet, layout, sift, type Finding, type Width } from './probe';
import { at, businessDate, closePool, insertAppointment, insertClient, resetData } from './seed';
import { ALLOW } from './ui-probe.allow';

/** One word, 60 letters: the longest kind of name a layout has to hold. Made up. */
const LONG_NAME = 'Mariafernanda'.repeat(5).slice(0, 60);
const LOGGED_OUT = { cookies: [], origins: [] };
/** A busy status is a `role="status"` line with one of the `*_loading` texts. */
const LOADING = new RegExp(
	`^(${Object.entries(es)
		.filter(([key]) => key.endsWith('_loading'))
		.map(([, text]) => text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'))
		.join('|')})$`
);

type Ids = { client: string; appointment: string };

/** The bootstrap lists a feed can empty. */
type BootList =
	'appointments' | 'due' | 'clients' | 'services' | 'attributes' | 'templates' | 'rules';

/** One source of server data on a screen. `empty` is what the screen says with no rows: from
 *  the real answer with `bootstrap` lists (or a list answer's `items`) emptied; `none` says why
 *  the feed has no empty state (one record, a form). */
type Feed = {
	name: string;
	/** The API paths of the feed. */
	api: RegExp;
	empty: { bootstrap?: BootList[]; text: string } | { none: string };
};

type Probe = {
	/** The address to open; may set up routing first (the setup wizard). */
	open: (page: Page, ids: Ids) => Promise<string>;
	/** A text that shows once the screen has its data. */
	ready: string;
	loggedOut?: true;
	sheets?: { name: string; open: (page: Page) => Promise<void> }[];
	/** The screen's data feeds, or why it has none. */
	feeds: Feed[] | { none: string };
};

const BOOTSTRAP = /^\/api\/v1\/bootstrap$/;
const MEDIA = /^\/api\/v1\/clients\/[^/]+\/media$/;
const fixed = (url: string) => (): Promise<string> => Promise.resolve(url);
const button = (name: string) => async (page: Page) => {
	await page.getByRole('button', { name, exact: true }).click();
};
const bootFeed = (name: string, empty: Feed['empty']): Feed => ({ name, api: BOOTSTRAP, empty });

/** Every route of `src/routes`, keyed like SvelteKit's route id. */
const PROBES: Record<string, Probe> = {
	'/': {
		open: fixed('/'),
		ready: LONG_NAME,
		feeds: [
			{
				name: 'agenda',
				api: /^\/api\/v1\/(bootstrap|appointments)$/,
				empty: { bootstrap: ['appointments', 'due'], text: es.agenda_empty_day }
			}
		]
	},
	'/enviar': {
		open: fixed('/enviar'),
		ready: LONG_NAME,
		feeds: [
			bootFeed('due list', { bootstrap: ['due'], text: es.due_today_none }),
			{ name: 'sent log', api: /^\/api\/v1\/reminders\/log$/, empty: { text: es.due_sent_empty } }
		]
	},
	'/clientas': {
		open: fixed('/clientas'),
		ready: LONG_NAME,
		feeds: [
			{
				name: 'clients',
				api: /^\/api\/v1\/(bootstrap|clients)$/,
				empty: { bootstrap: ['clients'], text: es.clients_empty_title }
			}
		]
	},
	'/clientas/[id]': {
		open: (_page, ids) => Promise.resolve(`/clientas/${ids.client}`),
		ready: LONG_NAME,
		sheets: [{ name: 'archive', open: button(es.client_archive) }],
		feeds: [
			{ name: 'client', api: /^\/api\/v1\/clients\/[^/]+$/, empty: { none: 'one record' } },
			{
				name: 'reminder log',
				api: /^\/api\/v1\/reminders\/log$/,
				empty: { text: es.client_reminders_empty }
			},
			{ name: 'photos', api: MEDIA, empty: { text: es.photos_empty_client } }
		]
	},
	'/clientas/nueva': {
		open: fixed('/clientas/nueva'),
		ready: es.client_name,
		feeds: [bootFeed('form data', { none: 'a form' })]
	},
	'/citas/[id]': {
		open: (_page, ids) => Promise.resolve(`/citas/${ids.appointment}`),
		ready: LONG_NAME,
		feeds: [
			{
				name: 'appointment',
				api: /^\/api\/v1\/appointments\/[^/]+$/,
				empty: { none: 'one record' }
			},
			{ name: 'photos', api: MEDIA, empty: { text: es.photos_empty } }
		]
	},
	'/citas/nueva': {
		open: fixed('/citas/nueva'),
		ready: 'Acrílico',
		sheets: [
			{
				name: 'quick-add client',
				open: async (page) => {
					await page.getByRole('button', { name: es.client_add, exact: true }).click();
				}
			}
		],
		feeds: [
			bootFeed('services', { bootstrap: ['services'], text: es.appointment_services_empty_title })
		]
	},
	'/ajustes': {
		open: fixed('/ajustes'),
		ready: es.settings_title,
		feeds: {
			none: 'compact: the fixed section list; wide: the Negocio screen, probed at /ajustes/negocio'
		}
	},
	'/ajustes/negocio': {
		open: fixed('/ajustes/negocio'),
		ready: es.business_save,
		feeds: [bootFeed('business', { none: 'one record' })]
	},
	'/ajustes/servicios': {
		open: fixed('/ajustes/servicios'),
		ready: 'Acrílico',
		sheets: [{ name: 'new service', open: button(es.services_new) }],
		feeds: [
			bootFeed('services', { bootstrap: ['services'], text: es.appointment_services_empty_title })
		]
	},
	'/ajustes/atributos': {
		open: fixed('/ajustes/atributos'),
		ready: 'Forma',
		sheets: [{ name: 'new attribute', open: button(es.attributes_new) }],
		feeds: [bootFeed('attributes', { bootstrap: ['attributes'], text: es.attributes_empty_title })]
	},
	'/ajustes/plantillas': {
		open: fixed('/ajustes/plantillas'),
		ready: 'Recordatorio de cita',
		feeds: [bootFeed('templates', { bootstrap: ['templates'], text: es.templates_empty_title })]
	},
	'/ajustes/plantillas/nueva': {
		open: fixed('/ajustes/plantillas/nueva'),
		ready: es.template_save,
		feeds: [bootFeed('form data', { none: 'a form' })]
	},
	'/ajustes/plantillas/[id]': {
		open: async (page) => {
			const day = businessDate(0);
			const res = await page.request.get(`/api/v1/bootstrap?from=${day}&to=${day}`);
			const body = (await res.json()) as { templates: { id: string }[] };
			const first = body.templates[0];
			if (first === undefined) {
				throw new Error('probe: setup made no template');
			}
			return `/ajustes/plantillas/${first.id}`;
		},
		ready: es.template_save,
		feeds: [bootFeed('template', { none: 'one record' })]
	},
	'/ajustes/recordatorios': {
		open: fixed('/ajustes/recordatorios'),
		ready: es.rules_new,
		sheets: [{ name: 'new rule', open: button(es.rules_new) }],
		feeds: [bootFeed('rules', { bootstrap: ['rules'], text: es.due_no_rules_title })]
	},
	'/ajustes/estado': {
		open: fixed('/ajustes/estado'),
		ready: es.status_version,
		feeds: [{ name: 'status', api: /^\/api\/v1\/status$/, empty: { none: 'one record' } }]
	},
	'/ajustes/exportar': {
		open: fixed('/ajustes/exportar'),
		ready: es.export_button,
		feeds: { none: 'a download link; the server builds the file' }
	},
	'/cambiar-clave': {
		open: fixed('/cambiar-clave'),
		ready: es.password_save,
		feeds: { none: 'a form with no server data' }
	},
	'/login': {
		open: fixed('/login'),
		ready: es.login_heading,
		loggedOut: true,
		feeds: { none: 'a form with no server data' }
	},
	'/setup': {
		// Setup is done by the setup project; the wizard shows again when the server says it is not.
		open: async (page) => {
			await page.route('**/api/v1/setup/status', async (route) => {
				const real = await route.fetch();
				await route.fulfill({ response: real, json: { needs_setup: true } });
			});
			return '/setup';
		},
		ready: es.setup_title,
		loggedOut: true,
		feeds: { none: 'the wizard asks; its first step has no server data' }
	}
};

/** The route ids of every `+page.svelte` under `src/routes`. */
function routesOnDisk(): string[] {
	const root = join(import.meta.dirname, '../../src/routes');
	return readdirSync(root, { recursive: true, encoding: 'utf8' })
		.filter((p) => p.split(sep).at(-1) === '+page.svelte')
		.map((p) =>
			`/${relative(root, join(root, p, '..'))
				.split(sep)
				.join('/')}`.replace(/\/$/, '')
		)
		.map((r) => (r === '' ? '/' : r))
		.sort();
}

function widthOf(project: string): Width {
	return project === 'probe-wide' ? 'wide' : 'compact';
}

const CLEAN = { open: [], stale: [] };

/** The findings no entry allows, and the entries for this screen that allowed nothing; a clean
 *  screen has neither (`CLEAN`). */
function verdict(findings: Finding[], route: string, project: string, state: string) {
	return sift(findings, ALLOW, { route, width: widthOf(project), state });
}

async function screenFindings(page: Page): Promise<Finding[]> {
	return [...(await axe(page)), ...(await layout(page))];
}

/** No finding when the state showed, else `missing`. */
function unless(shown: boolean, missing: Finding): Finding[] {
	return shown ? [] : [missing];
}

/** The screen's findings when the state it waited for showed, else `missing`. */
async function whenShown(page: Page, shown: boolean, missing: Finding): Promise<Finding[]> {
	return shown ? screenFindings(page) : [missing];
}

/** The screen's ready text where it shows (a hidden copy, like the compact client list, does not count). */
function readyText(page: Page, probe: Probe): Locator {
	return page.getByText(probe.ready, { exact: true }).filter({ visible: true }).first();
}

/** The API calls in flight that are not `feed`'s. */
function otherCalls(page: Page, feed: Feed): () => number {
	let open = 0;
	const other = (r: Request): boolean => {
		const path = new URL(r.url()).pathname;
		return path.startsWith('/api/') && !feed.api.test(path);
	};
	page.on('request', (r) => {
		open += Number(other(r));
	});
	const done = (r: Request): void => {
		open -= Number(other(r));
	};
	page.on('requestfinished', done);
	page.on('requestfailed', done);
	return () => open;
}

/** Whether `loc` is still shown after the screen has had time to render what it got. */
function stays(loc: Locator): Promise<boolean> {
	return loc
		.filter({ visible: true })
		.first()
		.waitFor({ state: 'hidden', timeout: 2_000 })
		.then(
			() => false,
			() => true
		);
}

/** Whether `loc` shows within the normal wait; a miss is a finding, not a test error. */
function shows(loc: Locator): Promise<boolean> {
	return loc
		.first()
		.waitFor({ state: 'visible', timeout: 5_000 })
		.then(
			() => true,
			() => false
		);
}

function feedMatch(feed: Feed): (url: URL) => boolean {
	return (url) => feed.api.test(url.pathname);
}

/** The real answer with the feed's lists emptied. */
async function answerEmpty(route: Route, lists: BootList[]): Promise<void> {
	const real = await route.fetch();
	const body = (await real.json()) as Record<string, unknown>;
	const boot = BOOTSTRAP.test(new URL(route.request().url()).pathname);
	const emptied = boot
		? Object.fromEntries(lists.map((k) => [k, []]))
		: 'items' in body
			? { items: [] }
			: {};
	await route.fulfill({ response: real, json: { ...body, ...emptied } });
}

/** A 500 the app wrote: its version header and an `internal` problem. */
async function answer500(route: Route): Promise<void> {
	const real = await route.fetch();
	await route.fulfill({
		response: real,
		status: 500,
		contentType: 'application/problem+json',
		json: { type: 'urn:open-customer:problem:internal', title: 'Algo salió mal', status: 500 }
	});
}

let ids: Ids = { client: '', appointment: '' };

test.use({ serviceWorkers: 'block' });

test.beforeEach(async () => {
	await resetData();
	const client = await insertClient({ name: LONG_NAME, phone: '+50688887777' });
	// Today, 30 minutes: the shortest box the day grid draws.
	const appointment = await insertAppointment({
		clientId: client,
		services: ['Acrílico'],
		startsAt: at(0, '10:00'),
		minutes: 30
	});
	// The 1-day "before" rule makes this one's message due today.
	await insertAppointment({ clientId: client, services: ['Relleno'], startsAt: at(1, '11:00') });
	ids = { client, appointment };
});

test.afterAll(closePool);

test('every route on disk has a probe entry, and every entry a route', () => {
	expect(Object.keys(PROBES).sort()).toEqual(routesOnDisk());
});

for (const [route, probe] of Object.entries(PROBES)) {
	test.describe(route, () => {
		test.use({ storageState: probe.loggedOut === true ? LOGGED_OUT : AUTH_FILE });

		test('passes axe and the layout probe', async ({ page }, info) => {
			await page.goto(await probe.open(page, ids));
			await expect(readyText(page, probe)).toBeVisible();

			expect(verdict(await screenFindings(page), route, info.project.name, 'base')).toEqual(CLEAN);
		});

		for (const sheet of probe.sheets ?? []) {
			test(`with the ${sheet.name} sheet open passes axe and the layout probe`, async ({
				page
			}, info) => {
				await page.goto(await probe.open(page, ids));
				await expect(readyText(page, probe)).toBeVisible();
				await sheet.open(page);
				await expect(page.getByRole('dialog')).toBeVisible();

				const found = [...(await screenFindings(page)), ...(await behindSheet(page))];
				expect(verdict(found, route, info.project.name, sheet.name)).toEqual(CLEAN);
			});
		}

		const feeds = Array.isArray(probe.feeds) ? probe.feeds : [];
		for (const feed of feeds) {
			test(`${feed.name}: a 500 shows an alert`, async ({ page }, info) => {
				await page.route(feedMatch(feed), answer500);
				await page.goto(await probe.open(page, ids));
				const found = await whenShown(page, await shows(page.getByRole('alert')), {
					check: 'error',
					target: feed.name,
					detail: 'no alert for a 500'
				});
				expect(verdict(found, route, info.project.name, `${feed.name} error`)).toEqual(CLEAN);
			});

			test(`${feed.name}: a slow answer shows a busy status`, async ({ page }, info) => {
				// Never answered: the screen waits for as long as the test looks.
				await page.route(feedMatch(feed), () => undefined);
				const others = otherCalls(page, feed);
				await page.goto(await probe.open(page, ids));
				// Every other feed answered first, so a loading line still there is this feed's.
				await shows(readyText(page, probe));
				await expect.poll(others).toBe(0);
				const busy = await stays(page.getByRole('status').filter({ hasText: LOADING }));
				await page.unrouteAll({ behavior: 'ignoreErrors' });

				const found = unless(busy, {
					check: 'busy',
					target: feed.name,
					detail: 'no loading line while the feed is out'
				});
				expect(verdict(found, route, info.project.name, `${feed.name} busy`)).toEqual(CLEAN);
			});

			const empty = feed.empty;
			if ('text' in empty) {
				test(`${feed.name}: no rows shows the empty state`, async ({ page }, info) => {
					await page.route(feedMatch(feed), (r) => answerEmpty(r, empty.bootstrap ?? []));
					await page.goto(await probe.open(page, ids));
					const found = await whenShown(
						page,
						await shows(page.getByText(empty.text, { exact: true })),
						{ check: 'empty', target: feed.name, detail: `no "${empty.text}"` }
					);
					expect(verdict(found, route, info.project.name, `${feed.name} empty`)).toEqual(CLEAN);
				});
			}
		}
	});
}
