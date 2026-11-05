<script lang="ts">
	import type { Attribute } from '../api/schemas';
	import { optionPath, type AttrValue } from '../booking';
	import { t } from '../i18n';
	import BottomSheet from './ui/BottomSheet.svelte';
	import Button from './ui/Button.svelte';
	import Icon from './icons/Icon.svelte';
	import SelectField from './SelectField.svelte';
	import TextField from './ui/TextField.svelte';

	// One attribute as an input (design doc item 70): text, number and date as plain fields, a flat
	// select as the native `<select>`, and a nested select or any multiselect as a drill-down in a
	// sheet: one list per level, and she can stop at any level. A value stays an id (or a list of ids).
	let {
		attr,
		value = $bindable(),
		error
	}: { attr: Attribute; value: AttrValue; error?: string | undefined } = $props();

	const text = $derived(typeof value === 'string' ? value : '');
	const ids = $derived(typeof value === 'string' ? (value === '' ? [] : [value]) : value);
	const multi = $derived(attr.type === 'multiselect');
	const nested = $derived(attr.options.some((o) => !o.archived && o.parent_id !== null));

	const summary = $derived(
		ids
			.map((id) =>
				optionPath(attr.options, id)
					.map((o) => o.label)
					.join(' ')
			)
			.join(', ')
	);

	const setText = (v: string): void => {
		value = v;
	};

	let open = $state(false);
	// The option whose children the sheet lists; null is the top level.
	let at = $state<string | null>(null);
	const level = $derived(attr.options.filter((o) => !o.archived && o.parent_id === at));
	const parent = $derived(attr.options.find((o) => o.id === at));
	const hasChildren = (id: string): boolean =>
		attr.options.some((o) => !o.archived && o.parent_id === id);

	function openSheet(): void {
		at = null;
		open = true;
	}

	function pick(id: string): void {
		if (multi) {
			value = ids.includes(id) ? ids.filter((x) => x !== id) : [...ids, id];
			return;
		}
		value = id;
		open = false;
	}

	function clear(): void {
		value = multi ? [] : '';
		open = false;
	}

	const selectOptions = $derived(
		attr.options
			.filter((o) => !o.archived || ids.includes(o.id))
			.map((o) => ({ id: o.id, label: o.label }))
	);
</script>

{#if attr.type === 'text'}
	<TextField label={attr.label} bind:value={() => text, setText} {error} />
{:else if attr.type === 'number'}
	<TextField label={attr.label} inputmode="decimal" bind:value={() => text, setText} {error} />
{:else if attr.type === 'date'}
	<TextField label={attr.label} type="date" bind:value={() => text, setText} {error} />
{:else if !multi && !nested}
	<SelectField
		label={attr.label}
		options={selectOptions}
		{...attr.required ? {} : { clearLabel: t('attr_empty_choice') }}
		bind:value={() => text, setText}
		{error}
	/>
{:else}
	<div class="flex flex-col gap-1.5">
		<span class="text-sm font-bold text-ink">{attr.label}</span>
		<button
			type="button"
			aria-haspopup="dialog"
			aria-label="{attr.label}: {summary === '' ? t('attr_choose') : summary}"
			class="flex h-12 w-full items-center gap-2 rounded-sm border bg-glass-strong px-4 text-left text-base hover:bg-accent-100 {error ===
			undefined
				? 'border-accent'
				: 'border-error'}"
			onclick={openSheet}
		>
			<span class="min-w-0 flex-1 truncate {summary === '' ? 'text-ink-muted' : 'text-ink'}">
				{summary === '' ? t('attr_choose') : summary}
			</span>
			<Icon name="chevron-right" class="size-4.5 text-ink-muted" />
		</button>
		{#if error !== undefined}
			<p class="text-sm text-error">{error}</p>
		{/if}
	</div>

	<BottomSheet
		{open}
		title={attr.label}
		onclose={() => {
			open = false;
		}}
	>
		{#if parent !== undefined}
			<button
				type="button"
				class="flex min-h-tap items-center gap-2 self-start rounded-pill pr-4 text-sm font-bold text-accent-700 hover:bg-accent-100"
				onclick={() => {
					at = parent.parent_id;
				}}
			>
				<Icon name="chevron-left" class="size-4.5" />
				{parent.label}
			</button>
		{/if}
		<ul class="flex flex-col">
			{#each level as option (option.id)}
				{@const chosen = ids.includes(option.id)}
				<li class="flex items-center">
					<button
						type="button"
						aria-pressed={chosen}
						class="flex min-h-tap flex-1 items-center gap-2 rounded-sm px-3 text-left text-base text-ink hover:bg-accent-100"
						onclick={() => {
							pick(option.id);
						}}
					>
						<span class="min-w-0 flex-1">{option.label}</span>
						{#if chosen}
							<Icon name="check" class="size-4.5 text-accent-700" />
						{/if}
					</button>
					{#if hasChildren(option.id)}
						<button
							type="button"
							title={t('attr_open_level', { label: option.label })}
							aria-label={t('attr_open_level', { label: option.label })}
							class="grid size-11 place-items-center rounded-pill text-ink-muted hover:bg-accent-100"
							onclick={() => {
								at = option.id;
							}}
						>
							<Icon name="chevron-right" class="size-4.5" />
						</button>
					{/if}
				</li>
			{:else}
				<li class="px-3 py-3 text-sm text-ink-muted">{t('attr_none')}</li>
			{/each}
		</ul>
		{#if multi}
			<Button
				variant="primary"
				onclick={() => {
					open = false;
				}}
			>
				{t('attr_done')}
			</Button>
		{/if}
		{#if !attr.required && ids.length > 0}
			<Button variant="secondary" onclick={clear}>{t('attr_clear')}</Button>
		{/if}
	</BottomSheet>
{/if}
