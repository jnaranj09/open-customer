// UI pipeline stage 7, runtime sensors (.claude/rules/ui.md): every route of the app, at the phone
// profile (`probe-compact`, 393 x 852) and the desktop size (`probe-wide`, 1440 x 900), through axe
// (WCAG 2.0 to 2.2 A/AA) and the layout probe (sideways scroll, 44 px tap targets, 16 px field font,
// cut text), with a 60-letter one-word client name seeded in a client, her appointments, a sent
// reminder and a photo. Every opened sheet gets the same checks
// and the page behind it must be inert. Every data feed of a screen is answered empty, with a 500
// and not at all, and the screen must show its empty text, an alert and a busy status. A route on
// disk with no entry here fails. Known findings are in `ui-probe.allow.ts`; an entry that names a
// route or state this table does not probe fails too.
import { readFileSync, readdirSync } from 'node:fs';
import { join, relative, sep } from 'node:path';
import {
	expect,
	request,
	test,
	type Locator,
	type Page,
	type Request,
	type Route
} from '@playwright/test';
import { BootstrapSchema } from '../../src/lib/api/schemas';
import es from '../../src/lib/i18n/es.json' with { type: 'json' };
import { AUTH_FILE, BASE_URL } from './env';
import {
	axe,
	behindSheet,
	layout,
	LONG_NAME,
	sift,
	tabFit,
	type Allow,
	type Finding,
	type Width
} from './probe';
import {
	at,
	businessDate,
	closePool,
	insertAppointment,
	insertClient,
	insertSentReminder,
	resetData
} from './seed';
import { fill } from './steps';
import { ALLOW } from './ui-probe.allow';

const LOGGED_OUT = { cookies: [], origins: [] };
const escapeRe = (text: string): string => text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
/** A busy status is a `role="status"` line with one of the `*_loading` texts. */
const LOADING = new RegExp(
	`^(${Object.entries(es)
		.filter(([key]) => key.endsWith('_loading'))
		.map(([, text]) => escapeRe(text))
		.join('|')})$`
);

type Ids = { client: string; appointment: string };

/** The bootstrap lists a feed can empty. */
type BootList =
	'appointments' | 'due' | 'clients' | 'services' | 'attributes' | 'templates' | 'rules';

/** One source of server data on a screen. `empty` is what the screen says with no rows: from
 *  the real answer with `bootstrap` lists (or a list answer's `items`) emptied; `none` says why
 *  the feed has no empty state (one record, a form). A screen whose two layouts show different
 *  views (Hoy: a day on compact, a week on wide) gives the text per width. */
type Feed = {
	name: string;
	/** The API paths of the feed. */
	api: RegExp;
	empty: { bootstrap?: BootList[]; text: string | Record<Width, string> } | { none: string };
	/** A feed that only adds a line: on an error and while it loads the line is hidden, with no
	 *  alert and no busy status. `answer` gives its empty answer; `open` makes the screen ask. */
	quiet?: Quiet;
};

type Quiet = {
	/** Every text the line can show. */
	line: RegExp;
	answer: (route: Route) => Promise<void>;
	open?: (page: Page) => Promise<void>;
};

/** A line that starts with one of `texts` (each up to its first `{placeholder}`). */
const startsWithOne = (texts: string[]): RegExp =>
	new RegExp(`^(${texts.map((text) => escapeRe(text.split('{')[0] ?? '')).join('|')})`);

/** The real answer with another status and body (its headers kept, the app's version among them). */
const answerWith =
	(status: number, json?: unknown) =>
	async (route: Route): Promise<void> => {
		const real = await route.fetch();
		await route.fulfill(
			json === undefined ? { response: real, status, body: '' } : { response: real, status, json }
		);
	};

type Probe = {
	/** The address to open; may set up routing first (the setup wizard). */
	open: (page: Page, ids: Ids) => Promise<string>;
	/** A text that shows once the screen has its data. */
	ready: string;
	loggedOut?: true;
	/** A signed-in screen outside the app shell (`hasShell`): no bottom bar on the phone. */
	noShell?: true;
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
				empty: {
					bootstrap: ['appointments', 'due'],
					text: { compact: es.agenda_empty_day, wide: es.agenda_empty_week }
				}
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
			{ name: 'photos', api: MEDIA, empty: { text: es.photos_empty_client } },
			{
				name: 'next message',
				api: /^\/api\/v1\/clients\/[^/]+\/next-reminder$/,
				empty: { text: es.client_next_none },
				quiet: {
					line: startsWithOne([
						es.client_next_reminder,
						es.client_next_today,
						es.client_next_late,
						es.client_next_none
					]),
					answer: answerWith(204)
				}
			}
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
	'/servicios': {
		open: fixed('/servicios'),
		ready: 'Acrílico',
		feeds: [
			bootFeed('services', { bootstrap: ['services'], text: es.appointment_services_empty_title })
		]
	},
	'/servicios/catalogo': {
		open: fixed('/servicios/catalogo'),
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
			const first = BootstrapSchema.parse(await res.json()).templates[0];
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
		feeds: [
			bootFeed('rules', { bootstrap: ['rules'], text: es.due_no_rules_title }),
			{
				name: 'rule preview',
				api: /^\/api\/v1\/reminder-rules\/preview$/,
				empty: { text: fill(es.rule_preview_none, { count: '0' }) },
				quiet: {
					line: startsWithOne([es.rule_preview, es.rule_preview_none]),
					answer: answerWith(200, { today_count: 0, next: null }),
					open: async (page) => {
						await button(es.rules_new)(page);
						const sheet = page.getByRole('dialog');
						await sheet
							.getByLabel(es.rule_kind, { exact: true })
							.selectOption({ label: es.rule_kind_come_back });
						await sheet.getByLabel(es.rule_template, { exact: true }).selectOption({ index: 1 });
						await sheet.getByLabel(es.rule_days_after, { exact: true }).fill('30');
						await sheet
							.getByLabel(es.rule_service, { exact: true })
							.selectOption({ label: es.rule_service_all });
					}
				}
			}
		]
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
		noShell: true,
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

/** The empty text a feed shows at the width of `project`. */
function emptyText(text: string | Record<Width, string>, project: string): string {
	return typeof text === 'string' ? text : text[widthOf(project)];
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

/** The bottom bar's checks, at the compact width only (the wide layout has the sidebar instead).
 *  A signed-in screen with no bar drawn is a finding there: `tabFit` alone would pass it. */
async function barFindings(page: Page, project: string, signedIn: boolean): Promise<Finding[]> {
	if (widthOf(project) !== 'compact') return [];
	const bar = page.getByRole('navigation', { name: es.nav_label }).filter({ visible: true });
	if (signedIn && (await bar.count()) === 0) {
		return [{ check: 'tab-fit', target: 'nav', detail: 'no bottom bar drawn' }];
	}
	return tabFit(page, es.nav_label);
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

/** Whether `loc` shows, then is still there 2 s later (a late first paint is not a miss). */
async function showsAndStays(loc: Locator): Promise<boolean> {
	return (await shows(loc)) && stays(loc);
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
		json: { type: 'urn:open-customer:problem:internal', title: es.error_internal, status: 500 }
	});
}

/** A made-up 64 x 64 JPEG (the booking spec's fixture), sent as both the photo and its thumbnail. */
const PHOTO = readFileSync('tests/e2e/fixtures/photo-gps.jpg');

/** Adds a photo to `appointmentId` through the API, as the logged-in owner. */
async function uploadPhoto(appointmentId: string): Promise<void> {
	const api = await request.newContext({
		baseURL: BASE_URL,
		storageState: AUTH_FILE,
		extraHTTPHeaders: { Origin: BASE_URL }
	});
	const jpeg = { name: 'foto.jpg', mimeType: 'image/jpeg', buffer: PHOTO };
	const res = await api.post('/api/v1/media', {
		multipart: { full: jpeg, thumb: jpeg, appointment_id: appointmentId }
	});
	await api.dispose();
	if (res.status() !== 201) {
		throw new Error(`probe: photo upload answered ${String(res.status())}`);
	}
}

/** The states the table probes on `route`: `base`, each sheet, and each feed's tests. */
function statesOf(probe: Probe): string[] {
	const feeds = Array.isArray(probe.feeds) ? probe.feeds : [];
	return [
		'base',
		...(probe.sheets ?? []).map((sheet) => sheet.name),
		...feeds.flatMap((f) => [
			`${f.name} error`,
			`${f.name} busy`,
			...('text' in f.empty ? [`${f.name} empty`] : [])
		])
	];
}

/** Whether `a` names a route and state this table probes (an entry that never meets its screen
 *  could never go stale). */
function probed(a: Allow): boolean {
	if ('witness' in a) {
		return a.witness in PROBES && a.state === undefined;
	}
	const probe = PROBES[a.route];
	return probe !== undefined && statesOf(probe).includes(a.state ?? 'base');
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
	// Today's sent message (Enviados, her Recordatorios) and a photo of today's visit.
	await insertSentReminder({
		appointmentId: appointment,
		body: `Hola ${LONG_NAME}, te esperamos hoy.`,
		actedAt: at(0, '08:00')
	});
	await uploadPhoto(appointment);
	ids = { client, appointment };
});

test.afterAll(closePool);

test('every route on disk has a probe entry, and every entry a route', () => {
	expect(Object.keys(PROBES).sort()).toEqual(routesOnDisk());
});

test('every allow-list entry names a probed route and state', () => {
	expect(ALLOW.filter((a) => !probed(a))).toEqual([]);
});

for (const [route, probe] of Object.entries(PROBES)) {
	test.describe(route, () => {
		test.use({ storageState: probe.loggedOut === true ? LOGGED_OUT : AUTH_FILE });

		test('passes axe and the layout probe', async ({ page }, info) => {
			await page.goto(await probe.open(page, ids));
			await expect(readyText(page, probe)).toBeVisible();

			const found = [
				...(await screenFindings(page)),
				...(await barFindings(
					page,
					info.project.name,
					probe.loggedOut !== true && probe.noShell !== true
				))
			];
			expect(verdict(found, route, info.project.name, 'base')).toEqual(CLEAN);
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
			if (feed.quiet !== undefined && 'text' in feed.empty) {
				quietTests(route, probe, feed, feed.quiet, feed.empty.text);
				continue;
			}

			test(`${feed.name}: a 500 shows an alert`, async ({ page }, info) => {
				await page.route(feedMatch(feed), answer500);
				await page.goto(await probe.open(page, ids));
				const alert = page.getByRole('alert').filter({ hasText: es.error_internal });
				const found = await whenShown(page, await shows(alert), {
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
				const loading = page.getByRole('status').filter({ hasText: LOADING });
				const busy = await showsAndStays(loading);
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
					const text = emptyText(empty.text, info.project.name);
					const found = await whenShown(page, await shows(page.getByText(text, { exact: true })), {
						check: 'empty',
						target: feed.name,
						detail: `no "${text}"`
					});
					expect(verdict(found, route, info.project.name, `${feed.name} empty`)).toEqual(CLEAN);
				});
			}
		}
	});
}

/** The states of a quiet feed: a 500 and no answer leave its line out, with no alert; its empty
 *  answer shows `empty`. Runs inside the route's `describe`. */
function quietTests(
	route: string,
	probe: Probe,
	feed: Feed,
	quiet: Quiet,
	empty: string | Record<Width, string>
): void {
	/** Opens the screen with `answer` on the feed and waits until the screen has asked. */
	const reach = async (page: Page, answer: (r: Route) => Promise<void> | void): Promise<void> => {
		const asked = page.waitForRequest((r) => feed.api.test(new URL(r.url()).pathname));
		await page.route(feedMatch(feed), answer);
		await page.goto(await probe.open(page, ids));
		await expect(readyText(page, probe)).toBeVisible();
		await quiet.open?.(page);
		await asked;
	};

	test(`${feed.name}: a 500 leaves the line out, with no alert`, async ({ page }, info) => {
		await reach(page, answer500);
		const shown = await shows(page.getByText(quiet.line).or(page.getByRole('alert')));
		const found = unless(!shown, {
			check: 'error',
			target: feed.name,
			detail: 'a line or an alert for a 500 of a quiet feed'
		});
		expect(verdict(found, route, info.project.name, `${feed.name} error`)).toEqual(CLEAN);
	});

	test(`${feed.name}: a slow answer leaves the line out`, async ({ page }, info) => {
		// Never answered: the screen waits for as long as the test looks.
		await reach(page, () => undefined);
		const shown = await shows(page.getByText(quiet.line));
		await page.unrouteAll({ behavior: 'ignoreErrors' });
		const found = unless(!shown, {
			check: 'busy',
			target: feed.name,
			detail: 'a line while a quiet feed is out'
		});
		expect(verdict(found, route, info.project.name, `${feed.name} busy`)).toEqual(CLEAN);
	});

	test(`${feed.name}: the empty answer shows the empty line`, async ({ page }, info) => {
		await reach(page, quiet.answer);
		const text = emptyText(empty, info.project.name);
		const found = await whenShown(page, await shows(page.getByText(text, { exact: true })), {
			check: 'empty',
			target: feed.name,
			detail: `no "${text}"`
		});
		expect(verdict(found, route, info.project.name, `${feed.name} empty`)).toEqual(CLEAN);
	});
}
