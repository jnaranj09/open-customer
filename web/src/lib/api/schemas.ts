// The parse site for API responses (rules/boundaries.md): client.ts runs every body through one of
// these once. Each schema is checked against the generated types, so `svelte-check` fails when the
// spec and a schema drift. Objects strip unknown fields on purpose: an installed app can be older
// than the server that answers it, and a new field must not break the old app.
import { z } from 'zod';
import type { components } from './schema';

type Schemas = components['schemas'];

const id = z.uuid();
const instant = z.iso.datetime({ offset: true });
const day = z.iso.date();

export const SettingsSchema = z.object({
	name: z.string(),
	timezone: z.string(),
	locale: z.string(),
	currency: z.string(),
	country_code: z.string(),
	accent_color: z.string(),
	push_hour: z.number().int(),
	has_custom_icon: z.boolean()
}) satisfies z.ZodType<Schemas['Settings']>;

export const OptionSchema = z.object({
	id,
	attribute_id: id,
	parent_id: z.nullable(id),
	label: z.string(),
	position: z.number().int(),
	archived: z.boolean()
}) satisfies z.ZodType<Schemas['Option']>;

export const AttributeSchema = z.object({
	id,
	key: z.string(),
	label: z.string(),
	scope: z.enum(['client', 'appointment']),
	type: z.enum(['text', 'number', 'date', 'select', 'multiselect']),
	required: z.boolean(),
	position: z.number().int(),
	archived: z.boolean(),
	options: z.array(OptionSchema)
}) satisfies z.ZodType<Schemas['Attribute']>;

export const ServiceSchema = z.object({
	id,
	name: z.string(),
	duration_min: z.number().int(),
	price_cents: z.nullable(z.number().int()),
	color: z.string(),
	position: z.number().int(),
	archived: z.boolean()
}) satisfies z.ZodType<Schemas['Service']>;

export const TemplateSchema = z.object({
	id,
	name: z.string(),
	body: z.string(),
	archived: z.boolean()
}) satisfies z.ZodType<Schemas['Template']>;

export const TemplatePreviewSchema = z.object({
	text: z.string(),
	unknown_vars: z.array(z.string())
}) satisfies z.ZodType<Schemas['TemplatePreview']>;

const RuleKindSchema = z.enum(['before_appointment', 'come_back']);

export const RuleSchema = z.object({
	id,
	kind: RuleKindSchema,
	service_id: z.nullable(id),
	return_service_id: z.nullable(id),
	offset_days: z.number().int(),
	template_id: id,
	applies_from: z.nullable(instant),
	archived: z.boolean()
}) satisfies z.ZodType<Schemas['Rule']>;

export const ClientSchema = z.object({
	id,
	name: z.string(),
	phone_e164: z.nullable(z.string()),
	phone_digits: z.nullable(z.string()),
	notes: z.nullable(z.string()),
	attrs: z.record(z.string(), z.unknown()),
	archived: z.boolean(),
	created_at: instant,
	updated_at: instant
}) satisfies z.ZodType<Schemas['Client']>;

export const ClientListSchema = z.object({
	items: z.array(ClientSchema)
}) satisfies z.ZodType<Schemas['ClientList']>;

const AppointmentServiceSchema = z.object({
	service_id: id,
	name: z.string(),
	price_cents: z.nullable(z.number().int()),
	duration_min: z.number().int()
}) satisfies z.ZodType<Schemas['AppointmentService']>;

export const AppointmentSchema = z.object({
	id,
	client_id: id,
	client_name: z.string(),
	starts_at: instant,
	ends_at: instant,
	status: z.enum(['scheduled', 'confirmed', 'cancelled', 'no_show']),
	done: z.boolean(),
	total_cents: z.number().int(),
	paid_cents: z.nullable(z.number().int()),
	attrs: z.record(z.string(), z.unknown()),
	notes: z.nullable(z.string()),
	services: z.array(AppointmentServiceSchema),
	created_at: instant,
	updated_at: instant
}) satisfies z.ZodType<Schemas['Appointment']>;

export const AppointmentListSchema = z.object({
	items: z.array(AppointmentSchema)
}) satisfies z.ZodType<Schemas['AppointmentList']>;

const BookingPromptSchema = z.object({
	appointment_id: id,
	starts_at: instant,
	services: z.array(z.string()),
	return_service_id: id,
	return_service_name: z.string()
}) satisfies z.ZodType<Schemas['BookingPrompt']>;

export const DueItemSchema = z.object({
	id: z.string(),
	rule_id: id,
	appointment_id: id,
	anchor_at: z.nullable(instant),
	kind: RuleKindSchema,
	client_id: id,
	client_name: z.string(),
	due_date: day,
	late: z.boolean(),
	text: z.string(),
	phone_digits: z.nullable(z.string()),
	wa_link: z.nullable(z.string()),
	mode: z.enum(['message', 'booking_prompt']),
	prompt: z.nullable(BookingPromptSchema)
}) satisfies z.ZodType<Schemas['DueItem']>;

export const LogItemSchema = z.object({
	rule_id: id,
	appointment_id: id,
	anchor_at: z.nullable(instant).exactOptional(),
	action: z.enum(['sent', 'skipped']),
	body: z.nullable(z.string()).exactOptional(),
	phone_digits: z.nullable(z.string()).exactOptional(),
	acted_at: instant,
	id,
	kind: RuleKindSchema,
	client_id: id,
	client_name: z.string()
}) satisfies z.ZodType<Schemas['LogItem']>;

export const LogListSchema = z.object({
	items: z.array(LogItemSchema)
}) satisfies z.ZodType<Schemas['LogList']>;

export const TemplateVarSchema = z.object({
	ns: z.string(),
	key: z.string(),
	label: z.string()
}) satisfies z.ZodType<Schemas['TemplateVar']>;

export const BootstrapSchema = z.object({
	server_time: instant,
	settings: SettingsSchema,
	attributes: z.array(AttributeSchema),
	services: z.array(ServiceSchema),
	templates: z.array(TemplateSchema),
	rules: z.array(RuleSchema),
	clients: z.array(ClientSchema),
	appointments: z.array(AppointmentSchema),
	due: z.array(DueItemSchema),
	template_vars: z.array(TemplateVarSchema),
	backup_red: z.boolean()
}) satisfies z.ZodType<Schemas['Bootstrap']>;

export const MeSchema = z.object({
	id,
	username: z.string(),
	must_change_password: z.boolean()
}) satisfies z.ZodType<Schemas['Me']>;

export const SetupStatusSchema = z.object({
	needs_setup: z.boolean()
}) satisfies z.ZodType<Schemas['SetupStatus']>;

export const SetupOptionsSchema = z.object({
	countries: z.array(z.string())
}) satisfies z.ZodType<Schemas['SetupOptions']>;

/** The prefix of every problem `type`; errors.ts strips it to get the short name. */
export const PROBLEM_PREFIX = 'urn:open-customer:problem:';

export const ProblemSchema = z.object({
	type: z.enum([
		'urn:open-customer:problem:validation',
		'urn:open-customer:problem:not-found',
		'urn:open-customer:problem:unauthorized',
		'urn:open-customer:problem:invalid-credentials',
		'urn:open-customer:problem:must-change-password',
		'urn:open-customer:problem:forbidden-origin',
		'urn:open-customer:problem:setup-closed',
		'urn:open-customer:problem:setup-token-invalid',
		'urn:open-customer:problem:overlap',
		'urn:open-customer:problem:duplicate-phone',
		'urn:open-customer:problem:invalid-transition',
		'urn:open-customer:problem:payload-too-large',
		'urn:open-customer:problem:unsupported-media-type',
		'urn:open-customer:problem:rate-limited',
		'urn:open-customer:problem:internal'
	]),
	title: z.string(),
	status: z.number().int(),
	errors: z.array(z.object({ field: z.string(), message: z.string() })).exactOptional(),
	retry_after_s: z.number().int().exactOptional(),
	clients: z.array(ClientSchema).exactOptional(),
	appointments: z.array(AppointmentSchema).exactOptional()
}) satisfies z.ZodType<Schemas['Problem']>;

export const PushKeySchema = z.object({
	public_key: z.string()
}) satisfies z.ZodType<Schemas['PushKey']>;

export const PushSubscriptionIdSchema = z.object({ id }) satisfies z.ZodType<
	Schemas['PushSubscriptionId']
>;

/** The JSON of a browser `PushSubscription`: the one parse site for it before it is posted. */
export const PushSubscriptionInputSchema = z.object({
	endpoint: z.string(),
	expirationTime: z.nullable(z.number()).exactOptional(),
	keys: z.object({ p256dh: z.string(), auth: z.string() })
}) satisfies z.ZodType<Schemas['PushSubscriptionInput']>;

export const MediaSchema = z.object({
	id,
	client_id: id,
	appointment_id: z.nullable(id),
	width: z.number().int(),
	height: z.number().int(),
	bytes: z.number().int(),
	created_at: instant,
	archived: z.boolean()
}) satisfies z.ZodType<Schemas['Media']>;

export const MediaListSchema = z.object({
	items: z.array(MediaSchema)
}) satisfies z.ZodType<Schemas['MediaList']>;

export type Appointment = z.infer<typeof AppointmentSchema>;
export type AttrOption = z.infer<typeof OptionSchema>;
export type Attribute = z.infer<typeof AttributeSchema>;
export type Service = z.infer<typeof ServiceSchema>;
export type Template = z.infer<typeof TemplateSchema>;
export type Client = z.infer<typeof ClientSchema>;
export type Media = z.infer<typeof MediaSchema>;
export type Settings = z.infer<typeof SettingsSchema>;
export type DueItem = z.infer<typeof DueItemSchema>;
export type LogItem = z.infer<typeof LogItemSchema>;
export type Bootstrap = z.infer<typeof BootstrapSchema>;
export type Problem = z.infer<typeof ProblemSchema>;
export type Me = z.infer<typeof MeSchema>;
