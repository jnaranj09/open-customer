<script lang="ts">
	import Callout from './ui/Callout.svelte';
	import TextButton from './ui/TextButton.svelte';
	import { tick } from 'svelte';
	import type { AttrOption } from '../api/schemas';
	import { cache } from '../cache/store.svelte';
	import { indexAtY, levelOf, reorder, type Positioned } from '../attributes';
	import { t } from '../i18n';
	import Button from './ui/Button.svelte';
	import WriteButton from './WriteButton.svelte';
	import Icon from './icons/Icon.svelte';
	import TextField from './ui/TextField.svelte';

	// The options of a select attribute (design doc item 77): one list per level with "+ Agregar
	// opción" at the end of each, a handle to drag a row, and a menu per row. The same moves work
	// without a pointer: the handle takes the up and down keys, and the menu has "Subir" and
	// "Bajar". The parent saves each change and answers whether it worked; the tree keeps an inline
	// form open when it did not.
	let {
		options,
		onadd,
		onrename,
		onmove,
		onarchive
	}: {
		/** Every option of the attribute; archived ones are not shown. */
		options: readonly AttrOption[];
		onadd: (parent: string | null, label: string) => Promise<boolean>;
		onrename: (id: string, label: string) => Promise<boolean>;
		onmove: (moves: Positioned[]) => Promise<boolean>;
		onarchive: (id: string) => Promise<boolean>;
	} = $props();

	type Form = { kind: 'add'; parent: string | null } | { kind: 'rename'; id: string };

	let menu = $state<string | null>(null);
	let form = $state<Form | null>(null);
	let text = $state('');
	let formError = $state<string | undefined>(undefined);
	let working = $state(false);
	let drag = $state<{ id: string; over: number } | null>(null);
	let root: HTMLElement | undefined = $state();

	// Every move and archive writes to the server, so they are off while it is (the form buttons
	// are `WriteButton`s). The reason shows once above the tree.
	const off = $derived(working || cache.offline);

	const hasChildren = (id: string): boolean => levelOf(options, id).length > 0;
	const adding = (parent: string | null): boolean => form?.kind === 'add' && form.parent === parent;
	const renaming = (id: string): boolean => form?.kind === 'rename' && form.id === id;

	function openForm(next: Form, initial: string): void {
		form = next;
		text = initial;
		formError = undefined;
		menu = null;
	}

	async function work(action: () => Promise<boolean>): Promise<boolean> {
		if (working) {
			return false;
		}
		working = true;
		try {
			return await action();
		} finally {
			working = false;
		}
	}

	async function submit(): Promise<void> {
		const current = form;
		if (current === null) {
			return;
		}
		if (text.trim() === '') {
			formError = t('option_label_required');
			return;
		}
		const ok = await work(() =>
			current.kind === 'add'
				? onadd(current.parent, text.trim())
				: onrename(current.id, text.trim())
		);
		if (ok) {
			form = null;
		}
	}

	/** Moves `id` to `to` among its siblings and puts the focus back on its handle. */
	async function move(parent: string | null, id: string, to: number): Promise<void> {
		const siblings = levelOf(options, parent);
		const moves = reorder(
			siblings,
			siblings.findIndex((o) => o.id === id),
			to
		);
		if (moves.length === 0) {
			return;
		}
		menu = null;
		if (await work(() => onmove(moves))) {
			await tick();
			root?.querySelector<HTMLElement>(`[data-handle="${id}"]`)?.focus();
		}
	}

	function rowMids(list: Element): number[] {
		return Array.from(list.querySelectorAll(':scope > li > [data-row]')).map((row) => {
			const box = row.getBoundingClientRect();
			return box.top + box.height / 2;
		});
	}

	function dragStart(event: PointerEvent, id: string): void {
		event.preventDefault();
		if (event.currentTarget instanceof HTMLElement) {
			event.currentTarget.setPointerCapture(event.pointerId);
		}
		menu = null;
		drag = { id, over: -1 };
	}

	function dragMove(event: PointerEvent): void {
		const list =
			event.currentTarget instanceof HTMLElement ? event.currentTarget.closest('ul') : null;
		if (drag !== null && list !== null) {
			drag = { id: drag.id, over: indexAtY(rowMids(list), event.clientY) };
		}
	}

	function dragEnd(parent: string | null): void {
		const current = drag;
		drag = null;
		if (current !== null && current.over >= 0) {
			void move(parent, current.id, current.over);
		}
	}

	function key(event: KeyboardEvent, parent: string | null, id: string, index: number): void {
		if (event.key === 'ArrowUp' || event.key === 'ArrowDown') {
			event.preventDefault();
			void move(parent, id, index + (event.key === 'ArrowUp' ? -1 : 1));
		}
	}
</script>

{#snippet inlineForm(label: string, confirm: string)}
	<Callout
		as="form"
		class="flex flex-col gap-2"
		onsubmit={(e: SubmitEvent) => {
			e.preventDefault();
			void submit();
		}}
	>
		<TextField {label} bind:value={text} autocomplete="off" error={formError} />
		<div class="grid grid-cols-2 gap-2">
			<WriteButton variant="strong" type="submit" busy={working}>{confirm}</WriteButton>
			<Button
				variant="glass"
				onclick={() => {
					form = null;
				}}>{t('cancel')}</Button
			>
		</div>
	</Callout>
{/snippet}

{#snippet level(parent: string | null)}
	{@const siblings = levelOf(options, parent)}
	<ul class="flex flex-col gap-1">
		{#each siblings as option, i (option.id)}
			<li>
				{#if renaming(option.id)}
					{@render inlineForm(t('option_label'), t('option_rename_confirm'))}
				{:else}
					<div
						data-row
						class="flex items-center rounded-md border {drag?.over === i
							? 'border-accent bg-accent-100'
							: 'border-glass-edge bg-glass'}"
					>
						<button
							type="button"
							data-handle={option.id}
							title={t('option_drag', { label: option.label })}
							aria-label={t('option_drag', { label: option.label })}
							disabled={off}
							class="grid size-11 shrink-0 cursor-grab touch-none place-items-center rounded-md text-ink-muted hover:bg-accent-100"
							onpointerdown={(e) => {
								dragStart(e, option.id);
							}}
							onpointermove={dragMove}
							onpointerup={() => {
								dragEnd(parent);
							}}
							onpointercancel={() => {
								drag = null;
							}}
							onkeydown={(e) => {
								key(e, parent, option.id, i);
							}}
						>
							<Icon name="grip-vertical" class="size-5" />
						</button>
						<span class="min-w-0 flex-1 truncate text-base text-ink">{option.label}</span>
						<button
							type="button"
							title={t('option_menu', { label: option.label })}
							aria-label={t('option_menu', { label: option.label })}
							aria-expanded={menu === option.id}
							disabled={working}
							class="grid size-11 shrink-0 place-items-center rounded-md text-ink-muted hover:bg-accent-100"
							onclick={() => {
								menu = menu === option.id ? null : option.id;
							}}
						>
							<Icon name="ellipsis" class="size-5" />
						</button>
					</div>
					{#if menu === option.id}
						<div role="group" aria-label={option.label} class="mt-1 flex flex-wrap gap-1 pl-2">
							<TextButton
								variant="accent"
								disabled={off || i === 0}
								onclick={() => {
									void move(parent, option.id, i - 1);
								}}>{t('option_up')}</TextButton
							>
							<TextButton
								variant="accent"
								disabled={off || i === siblings.length - 1}
								onclick={() => {
									void move(parent, option.id, i + 1);
								}}>{t('option_down')}</TextButton
							>
							<TextButton
								variant="accent"
								onclick={() => {
									openForm({ kind: 'rename', id: option.id }, option.label);
								}}>{t('option_rename')}</TextButton
							>
							<TextButton
								variant="accent"
								onclick={() => {
									openForm({ kind: 'add', parent: option.id }, '');
								}}>{t('option_add_inside')}</TextButton
							>
							<TextButton
								variant="danger"
								disabled={off}
								onclick={() => {
									menu = null;
									void work(() => onarchive(option.id));
								}}>{t('option_archive')}</TextButton
							>
						</div>
					{/if}
				{/if}
				{#if hasChildren(option.id) || adding(option.id)}
					<div
						role="group"
						aria-label={t('option_level', { label: option.label })}
						class="mt-1 ml-5 border-l border-glass-edge pl-3"
					>
						{@render level(option.id)}
					</div>
				{/if}
			</li>
		{/each}
	</ul>
	{#if adding(parent)}
		{@render inlineForm(t('option_label'), t('option_add_confirm'))}
	{:else}
		<TextButton
			variant="accent"
			onclick={() => {
				openForm({ kind: 'add', parent }, '');
			}}
			class="mt-1 self-start">{t('option_add')}</TextButton
		>
	{/if}
{/snippet}

<div bind:this={root} class="flex flex-col gap-1">
	{#if cache.offline}
		<p class="text-sm text-ink-muted">{t('offline_cannot_save')}</p>
	{/if}
	{@render level(null)}
</div>
