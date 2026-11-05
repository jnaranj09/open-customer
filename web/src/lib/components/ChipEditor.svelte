<script lang="ts">
	import { untrack } from 'svelte';
	import type { Attribute } from '../api/schemas';
	import {
		bodyProblems,
		chipKnown,
		chipLabel,
		fallbackError,
		parseBody,
		piecesFromDom,
		serializeBody,
		type BodyProblem,
		type Piece
	} from '../chips';
	import {
		caretIn,
		chipElement,
		chipOf,
		clearIfEmpty,
		domNodes,
		fillField,
		insertAt,
		paintChip,
		removeChip,
		setChipFallback,
		syncSentinel
	} from '../chipsDom';
	import { t, type MessageKey } from '../i18n';
	import BottomSheet from './BottomSheet.svelte';
	import Button from './Button.svelte';
	import TemplatePreview from './TemplatePreview.svelte';
	import TextField from './TextField.svelte';

	// The message editor (design doc item 70): a contenteditable field where each variable is a
	// chip that cannot be edited inside, a row of chips to insert variables, a tap on a chip to
	// set "Si no tiene", and the live preview. The browser's DOM is the source while she types; the
	// body (template syntax) is read back from it after each change and handed to `onchange`. `value`
	// is only the start: the caller keys the editor on what it edits. The DOM <-> pieces mapping is
	// `lib/chips.ts`; this file holds the browser glue (caret, paste, Enter, IME).
	let {
		value,
		vars,
		attributes,
		label,
		error,
		onchange
	}: {
		value: string;
		/** The variables the chip row offers (`template_vars` of /bootstrap). */
		vars: readonly { ns: string; key: string; label: string }[];
		/** Every attribute, archived ones too, for the label of a chip the row no longer offers. */
		attributes: readonly Pick<Attribute, 'scope' | 'key' | 'label'>[];
		label: string;
		error?: string | undefined;
		/** The body, and what is wrong with it (the caller blocks saving while there is anything). */
		onchange: (body: string, problems: BodyProblem[]) => void;
	} = $props();

	const PROBLEM_TEXT = {
		braces: 'template_problem_braces',
		unknown: 'template_problem_unknown'
	} as const satisfies Record<BodyProblem, MessageKey>;

	const fieldId = $props.id();
	let field: HTMLElement | undefined = $state();
	let body = $state(untrack(() => value));
	let problems = $state<BodyProblem[]>([]);
	let composing = false;
	// The last caret inside the field: a tap on a chip button moves focus away from it.
	let caret: Range | null = null;

	let editing = $state<HTMLElement | null>(null);
	let editingTitle = $state('');
	let draft = $state('');
	let attempted = $state(false);
	const draftError = $derived(attempted ? fallbackError(draft) : null);

	const labelOf = (ns: string, key: string): string => chipLabel(ns, key, vars, attributes);

	const newChip = (ns: string, key: string, fallback: string | null): HTMLElement =>
		chipElement(ns, key, fallback, labelOf(ns, key), t('template_chip_hint'));

	function readPieces(): Piece[] {
		return field === undefined ? [] : piecesFromDom(domNodes(field.childNodes));
	}

	function changed(): void {
		if (field === undefined) {
			return;
		}
		clearIfEmpty(field);
		if (!composing) {
			syncSentinel(field);
		}
		const pieces = readPieces();
		body = serializeBody(pieces);
		problems = bodyProblems(pieces, (ns, key) => chipKnown(ns, key, vars, attributes));
		onchange(body, problems);
	}

	// Initial content: rendered once when the field mounts (the caller keys the editor on its record).
	function fill(node: HTMLElement): void {
		// Untracked as a whole: a change of `vars` or `value` later must not fill the field again.
		untrack(() => {
			fillField(
				node,
				parseBody(value).map((p) =>
					p.kind === 'text' ? document.createTextNode(p.text) : newChip(p.ns, p.key, p.fallback)
				)
			);
		});
	}

	function rememberCaret(): void {
		if (field !== undefined) {
			caret = caretIn(field) ?? caret;
		}
	}

	function insertNode(node: Node): void {
		if (field === undefined) {
			return;
		}
		// The live caret when it is in the field; the last one seen when focus has left it (the
		// stored one can lag a keystroke behind, as `selectionchange` is asynchronous).
		caret = insertAt(field, caretIn(field) ?? caret, node);
		changed();
	}

	// A pasted body (a copied message) turns its variables into chips, as a saved one does when it
	// is opened: text and saved body then agree. Any other braces stay text and are reported.
	function pasteText(text: string): void {
		const fragment = document.createDocumentFragment();
		for (const p of parseBody(text.replaceAll('\r\n', '\n').replaceAll('\r', '\n'))) {
			fragment.appendChild(
				p.kind === 'text' ? document.createTextNode(p.text) : newChip(p.ns, p.key, p.fallback)
			);
		}
		insertNode(fragment);
	}

	const insertText = (text: string): void => {
		insertNode(document.createTextNode(text.replaceAll('\r\n', '\n').replaceAll('\r', '\n')));
	};

	function onbeforeinput(event: InputEvent): void {
		if (event.isComposing) {
			return;
		}
		// Enter becomes a plain newline (the field is pre-wrap), never a block or a <br> of the browser's making.
		if (event.inputType === 'insertParagraph' || event.inputType === 'insertLineBreak') {
			event.preventDefault();
			insertText('\n');
		} else if (event.inputType.startsWith('format')) {
			// Bold, italic and the like (the iOS edit menu): the message is plain text.
			event.preventDefault();
		}
	}

	function onpaste(event: ClipboardEvent): void {
		event.preventDefault();
		pasteText(event.clipboardData?.getData('text/plain') ?? '');
	}

	function openFallback(el: HTMLElement): void {
		editing = el;
		editingTitle = labelOf(el.dataset['ns'] ?? '', el.dataset['key'] ?? '');
		draft = el.dataset['fallback'] ?? '';
		attempted = false;
	}

	function saveFallback(): void {
		attempted = true;
		if (editing === null || fallbackError(draft) !== null) {
			return;
		}
		setChipFallback(editing, draft);
		paintChip(editing, editingTitle, t('template_chip_hint'));
		editing = null;
		changed();
	}

	function dropChip(): void {
		if (editing !== null) {
			removeChip(editing);
		}
		editing = null;
		changed();
	}
</script>

<svelte:document onselectionchange={rememberCaret} />

<div class="flex flex-col gap-4">
	<div class="flex flex-col gap-1.5">
		<span id="{fieldId}-label" class="text-sm font-bold text-ink">{label}</span>
		<!-- A textbox with its own keyboard handling, not an interactive widget to tab through. -->
		<div
			bind:this={field}
			{@attach fill}
			role="textbox"
			tabindex="0"
			aria-multiline="true"
			aria-labelledby="{fieldId}-label"
			aria-invalid={error !== undefined || problems.length > 0}
			contenteditable="true"
			spellcheck="true"
			data-placeholder={t('template_message_placeholder')}
			class="min-h-40 w-full rounded-sm border bg-glass-strong px-4 py-3 text-base leading-loose wrap-break-word whitespace-pre-wrap text-ink empty:before:text-ink-muted empty:before:content-[attr(data-placeholder)] focus:border-accent {error ===
				undefined && problems.length === 0
				? 'border-glass-edge'
				: 'border-error'}"
			{onbeforeinput}
			{onpaste}
			oninput={changed}
			ondrop={(event) => {
				event.preventDefault();
			}}
			oncompositionstart={() => {
				composing = true;
			}}
			oncompositionend={() => {
				composing = false;
				changed();
			}}
			onblur={rememberCaret}
			onclick={(event) => {
				const chip = chipOf(event);
				if (chip !== null) {
					openFallback(chip);
				}
			}}
			onkeydown={(event) => {
				const chip = chipOf(event);
				if (chip !== null && (event.key === 'Enter' || event.key === ' ')) {
					event.preventDefault();
					openFallback(chip);
				}
			}}
		></div>
		{#each [...(error === undefined ? [] : [error]), ...problems.map( (p) => t(PROBLEM_TEXT[p]) )] as message (message)}
			<p class="text-sm text-error">{message}</p>
		{/each}
	</div>

	<div class="flex flex-col gap-2">
		<p class="text-sm font-bold text-ink">{t('template_add_variable')}</p>
		<ul class="flex flex-wrap gap-2">
			{#each vars as v (`${v.ns}.${v.key}`)}
				<li>
					<button
						type="button"
						class="min-h-tap rounded-pill border border-accent-300 bg-accent-100 px-4 text-sm font-bold text-accent-700 hover:bg-accent-300"
						onpointerdown={(event) => {
							// Keeps the caret in the field while the chip is inserted.
							event.preventDefault();
						}}
						onclick={() => {
							rememberCaret();
							insertNode(newChip(v.ns, v.key, null));
						}}
					>
						+ {v.label}
					</button>
				</li>
			{/each}
		</ul>
	</div>

	<TemplatePreview {body} />
</div>

<BottomSheet
	open={editing !== null}
	title={editingTitle}
	onclose={() => {
		editing = null;
	}}
>
	<TextField
		label={t('template_fallback_label')}
		autocomplete="off"
		hint={t('template_fallback_hint')}
		error={draftError === null ? undefined : t('template_fallback_quote')}
		bind:value={draft}
	/>
	<Button variant="primary" onclick={saveFallback}>{t('template_fallback_done')}</Button>
	<Button variant="ghost" onclick={dropChip}>{t('template_chip_remove')}</Button>
</BottomSheet>
