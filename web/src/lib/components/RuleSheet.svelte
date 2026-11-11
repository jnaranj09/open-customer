<script lang="ts">
	import { untrack } from 'svelte';
	import { api } from '../api/app';
	import { errorText, type ApiError } from '../api/errors';
	import type { Rule, Service, Template } from '../api/schemas';
	import { cache } from '../cache/store.svelte';
	import { t, type MessageKey } from '../i18n';
	import { byPosition } from '../order';
	import {
		ALL_SERVICES,
		handoverAsks,
		isGeneral,
		overdueChoice,
		previewInput,
		rulePatch,
		rulePreviewText,
		ruleRequest,
		type RuleField,
		type RuleForm,
		type RuleInput,
		type RulePatch,
		type RuleKind
	} from '../rules';
	import type { Option } from '../setup';
	import { submitOnce } from '../submit';
	import ActionButton from './ActionButton.svelte';
	import OverdueChoice from './OverdueChoice.svelte';
	import BottomSheet from './ui/BottomSheet.svelte';
	import Button from './ui/Button.svelte';
	import SaveButton from './SaveButton.svelte';
	import SelectField from './ui/SelectField.svelte';
	import TextField from './ui/TextField.svelte';
	import { PREVIEW_DEBOUNCE_MS } from '../preview';

	// Add or edit one reminder rule in a sheet (design doc item 77, CUJ 2 steps 3 and 4). A
	// come-back change that can add items first asks "¿Incluir atrasados?", and so does the archive
	// of a rule for one service while a general rule takes its visits (reminders-and-replies design,
	// "Handover"): the question replaces the form in the same sheet, and "Cancelar" or Escape on it
	// goes back to the form. A rule never changes kind or service, so an open rule shows those as
	// text. "Todos los servicios" makes a general rule, which has no service to book.
	let {
		rule,
		rules,
		templates,
		services,
		onclose,
		ondone
	}: {
		/** The rule being edited; none means a new one. */
		rule?: Rule;
		/** Every rule, for the handover question. */
		rules: readonly Rule[];
		templates: readonly Template[];
		services: readonly Service[];
		onclose: () => void;
		/** Told after a save, an archive or a restore, with the message to show. */
		ondone: (message: MessageKey) => void;
	} = $props();

	// What a tap on Save, Archive or Restore will send, held while she answers the overdue question.
	type Action = { create: RuleInput } | { update: RulePatch; message: MessageKey };

	let kindId = $state<string>(untrack(() => rule?.kind ?? 'before_appointment'));
	let templateId = $state(untrack(() => rule?.template_id ?? ''));
	let days = $state(untrack(() => (rule === undefined ? '' : String(rule.offset_days))));
	let serviceId = $state(untrack(() => rule?.service_id ?? ''));
	let returnId = $state(untrack(() => rule?.return_service_id ?? ''));
	let errors = $state<Partial<Record<RuleField, MessageKey>>>({});
	let failure = $state<ApiError | null>(null);
	let busy = $state(false);
	let asking = $state<Action | null>(null);

	const kind = $derived<RuleKind>(kindId === 'come_back' ? 'come_back' : 'before_appointment');
	const archivedTemplates = $derived(new Set(templates.filter((x) => x.archived).map((x) => x.id)));

	// The pickable rows, plus the one the rule holds now when it is archived, marked as such.
	const templateOptions = $derived<Option[]>(
		templates
			.filter((x) => !x.archived || x.id === rule?.template_id)
			.map((x) => ({
				id: x.id,
				label: x.archived ? t('rule_archived_suffix', { name: x.name }) : x.name
			}))
	);
	const serviceOptions = (current: string | null | undefined): Option[] =>
		byPosition(
			services.filter((x) => !x.archived || x.id === current),
			(x) => x.name
		).map((x) => ({
			id: x.id,
			label: x.archived ? t('rule_archived_suffix', { name: x.name }) : x.name
		}));
	const kindOptions = $derived<Option[]>([
		{ id: 'before_appointment', label: t('rule_kind_before') },
		{ id: 'come_back', label: t('rule_kind_come_back') }
	]);
	const serviceName = $derived(
		rule !== undefined && isGeneral(rule)
			? t('rule_service_all')
			: (services.find((x) => x.id === rule?.service_id)?.name ?? '…')
	);
	// A general rule (new with "Todos los servicios", or open) has no service to book.
	const general = $derived(rule === undefined ? serviceId === ALL_SERVICES : isGeneral(rule));

	/** The form as `ruleRequest` reads it. */
	function form(): RuleForm {
		return {
			kind,
			templateId,
			days,
			serviceId: rule === undefined ? serviceId : (rule.service_id ?? ALL_SERVICES),
			returnId: general ? '' : returnId
		};
	}

	// The preview of a come-back rule (reminders-and-replies design, CUJ R7): asked for while the
	// form is valid, after the template preview's pause. An archived rule sends nothing, so it has
	// none. On an error or offline the line is hidden.
	const previewBody = $derived.by(() => {
		const request = ruleRequest(form());
		return request.ok && rule?.archived !== true ? previewInput(request.body, rule?.id) : null;
	});
	let previewLine = $state<string | null>(null);
	// Timers and the server are outside systems, so this is an effect. A change before the answer
	// comes marks that answer stale: it is dropped, never shown after a newer one.
	$effect(() => {
		const body = previewBody;
		if (body === null) {
			previewLine = null;
			return;
		}
		let stale = false;
		const timer = setTimeout(() => {
			void api.previewRule(body).then((result) => {
				if (!stale) {
					previewLine = result.ok ? rulePreviewText(result.data) : null;
				}
			});
		}, PREVIEW_DEBOUNCE_MS);
		return () => {
			stale = true;
			clearTimeout(timer);
		};
	});

	const fieldError = (field: RuleField, key: string): string | undefined => {
		const own = errors[field];
		return own === undefined
			? failure?.kind === 'problem'
				? failure.fields[key]
				: undefined
			: t(own);
	};
	// A problem that names one of the form's fields shows under it; any other shows once, below.
	const FIELD_KEYS = ['template_id', 'offset_days', 'service_id', 'return_service_id'];
	const otherError = $derived.by(() => {
		const failed = failure;
		if (failed === null) {
			return null;
		}
		const onField = failed.kind === 'problem' && FIELD_KEYS.some((key) => key in failed.fields);
		return onField ? null : errorText(failed);
	});

	const send = submitOnce(
		async (action: Action, include: boolean | undefined) => {
			failure = null;
			const withOverdue = include === undefined ? {} : { include_overdue: include };
			const result =
				'create' in action
					? await api.createRule({ ...action.create, ...withOverdue })
					: rule === undefined
						? null
						: await api.updateRule(rule.id, { ...action.update, ...withOverdue });
			if (result === null) {
				return;
			}
			if (!result.ok) {
				failure = result.error;
				asking = null;
				return;
			}
			await cache.refresh();
			ondone('create' in action ? 'rule_created' : action.message);
		},
		(pending) => {
			busy = pending;
		}
	);

	// Moves focus to the first control of the step that just appeared.
	function focusFirst(node: HTMLElement): void {
		node.querySelector<HTMLElement>('select, input, button')?.focus();
	}

	/** Sends `action` now, or asks the overdue question first when it can add items to a come-back
	 *  rule (a "before" rule includes them without asking; see `overdueChoice`). */
	function go(action: Action): void {
		const choice =
			'create' in action
				? overdueChoice(null, action.create)
				: rule === undefined
					? 'none'
					: overdueChoice(rule, action.update, archivedTemplates);
		const handover =
			'update' in action &&
			action.update.archived === true &&
			rule !== undefined &&
			handoverAsks({ kind: 'rule_archive', id: rule.id }, rules, templates, services);
		if (choice === 'ask' || handover) {
			asking = action;
		} else {
			void send.run(action, choice === 'include' ? true : undefined);
		}
	}

	function save(): void {
		failure = null;
		const request = ruleRequest(form());
		if (!request.ok) {
			errors = request.errors;
			return;
		}
		errors = {};
		if (rule === undefined) {
			go({ create: request.body });
			return;
		}
		const update = rulePatch(rule, request.body);
		if (Object.keys(update).length === 0) {
			ondone('rule_saved');
			return;
		}
		go({ update, message: 'rule_saved' });
	}
</script>

<!-- One sheet for both steps, so a sheet never opens over another: the question replaces the form
     and "Cancelar" or Escape on it goes back to the form with her answers kept. -->
<BottomSheet
	open
	title={asking === null
		? rule === undefined
			? t('rule_new_title')
			: t('rule_edit_title')
		: t('overdue_title')}
	onclose={() => {
		if (asking === null) {
			onclose();
		} else {
			asking = null;
		}
	}}
>
	{#if asking === null}
		<div class="flex flex-col gap-3" {@attach focusFirst}>
			<form
				class="flex flex-col gap-4"
				onsubmit={(e) => {
					e.preventDefault();
					save();
				}}
			>
				{#if rule === undefined}
					<SelectField label={t('rule_kind')} bind:value={kindId} options={kindOptions} />
				{/if}
				<SelectField
					label={t('rule_template')}
					bind:value={templateId}
					options={templateOptions}
					error={fieldError('template', 'template_id')}
				/>
				<TextField
					label={kind === 'before_appointment' ? t('rule_days_before') : t('rule_days_after')}
					bind:value={days}
					inputmode="numeric"
					error={fieldError('days', 'offset_days')}
				/>
				{#if kind === 'come_back'}
					{#if rule === undefined}
						<SelectField
							label={t('rule_service')}
							bind:value={serviceId}
							options={[
								{ id: ALL_SERVICES, label: t('rule_service_all') },
								...serviceOptions(undefined)
							]}
							error={fieldError('service', 'service_id')}
						/>
					{:else}
						<p class="text-sm text-ink-muted">
							{t('rule_service')}: <span class="font-bold text-ink">{serviceName}</span>
						</p>
					{/if}
					{#if !general}
						<SelectField
							label={t('rule_return')}
							bind:value={returnId}
							options={serviceOptions(rule?.return_service_id)}
							clearLabel={t('rule_return_same')}
							error={failure?.kind === 'problem' ? failure.fields['return_service_id'] : undefined}
						/>
					{/if}
				{/if}
				{#if previewLine !== null}
					<p role="status" class="text-sm text-ink-muted">{previewLine}</p>
				{/if}
				{#if otherError !== null}
					<p role="alert" class="text-sm text-error">{otherError}</p>
				{/if}
				<SaveButton {busy}>{t('rule_save')}</SaveButton>
			</form>
			{#if rule !== undefined}
				{#if rule.archived}
					<div class="flex">
						<ActionButton
							variant="glass"
							{busy}
							onclick={() => {
								go({ update: { archived: false }, message: 'rule_restored' });
							}}
						>
							{t('rule_restore')}
						</ActionButton>
					</div>
				{:else}
					<p class="text-sm text-ink-muted">{t('rule_archive_body')}</p>
					<div class="flex">
						<ActionButton
							variant="glass"
							{busy}
							onclick={() => {
								go({ update: { archived: true }, message: 'rule_archived' });
							}}
						>
							{t('rule_archive')}
						</ActionButton>
					</div>
				{/if}
			{/if}
			<Button variant="ghost" onclick={onclose}>{t('cancel')}</Button>
		</div>
	{:else}
		<OverdueChoice
			{busy}
			onanswer={(include: boolean) => {
				if (asking !== null) {
					void send.run(asking, include);
				}
			}}
			oncancel={() => {
				asking = null;
			}}
		/>
	{/if}
</BottomSheet>
