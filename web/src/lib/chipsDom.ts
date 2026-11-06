// The browser side of the ChipEditor: the chip elements, reading the field's DOM into `DomNode`s,
// and the caret. All the rules that can be wrong are in `chips.ts` (pure, tested); this file only
// moves between the DOM and those types, so it has no logic of its own to test without a browser.
import { chipText, type DomNode } from './chips';
import { TOKEN_CHIP_CLASSES } from './components/ui/chipStyle';

const isChip = (node: Node | null): boolean =>
	node instanceof HTMLElement && node.dataset['chip'] !== undefined;
const isSentinel = (node: Node | null): boolean =>
	node instanceof HTMLElement && node.dataset['sentinel'] !== undefined;

/** The chip element for a variable. `fallback` null is "no fallback"; an empty string is kept. */
export function chipElement(
	ns: string,
	key: string,
	fallback: string | null,
	label: string,
	hint: string
): HTMLElement {
	const el = document.createElement('span');
	el.contentEditable = 'false';
	el.setAttribute('role', 'button');
	el.tabIndex = 0;
	el.dataset['chip'] = '';
	el.dataset['ns'] = ns;
	el.dataset['key'] = key;
	if (fallback !== null) {
		el.dataset['fallback'] = fallback;
	}
	el.className = TOKEN_CHIP_CLASSES;
	paintChip(el, label, hint);
	return el;
}

/** Sets the words a chip shows (its label and its fallback) and its accessible name. */
export function paintChip(el: HTMLElement, label: string, hint: string): void {
	const text = chipText(label, el.dataset['fallback'] ?? null);
	el.textContent = text;
	el.setAttribute('aria-label', `${text}. ${hint}`);
}

/** Sets or clears (empty text) the fallback of a chip; `paintChip` shows it. */
export function setChipFallback(el: HTMLElement, fallback: string): void {
	if (fallback === '') {
		el.removeAttribute('data-fallback');
	} else {
		el.dataset['fallback'] = fallback;
	}
}

/** The chip an event came from, or null. */
export function chipOf(event: Event): HTMLElement | null {
	return event.target instanceof Element ? event.target.closest<HTMLElement>('[data-chip]') : null;
}

/** The field's nodes as `DomNode`s: chips, text, `<br>`, and the blocks a browser wraps lines in.
 *  The caret sentinel is skipped. */
export function domNodes(list: NodeListOf<ChildNode>): DomNode[] {
	const out: DomNode[] = [];
	for (const node of list) {
		if (node instanceof Text) {
			out.push({ kind: 'text', text: node.data });
		} else if (!(node instanceof HTMLElement) || isSentinel(node)) {
			continue;
		} else if (isChip(node)) {
			out.push({
				kind: 'chip',
				ns: node.dataset['ns'] ?? '',
				key: node.dataset['key'] ?? '',
				fallback: node.dataset['fallback'] ?? null
			});
		} else if (node.tagName === 'BR') {
			out.push({ kind: 'br' });
		} else if (node.tagName === 'DIV' || node.tagName === 'P') {
			out.push({ kind: 'block', children: domNodes(node.childNodes) });
		} else {
			out.push(...domNodes(node.childNodes));
		}
	}
	return out;
}

/** A chip or a trailing newline at the very end needs something after it for the caret to sit
 *  on: a `<br>` the reader skips. Adds or removes it as needed. */
export function syncSentinel(field: HTMLElement): void {
	const last = field.lastChild;
	const sentinel = isSentinel(last) ? last : null;
	const tail = sentinel === null ? last : sentinel.previousSibling;
	const needed = isChip(tail) || (tail instanceof Text && tail.data.endsWith('\n'));
	if (needed && sentinel === null) {
		const br = document.createElement('br');
		br.dataset['sentinel'] = '';
		field.appendChild(br);
	} else if (!needed) {
		sentinel?.remove();
	}
}

/** Empties a field a browser left holding only a `<br>` or an empty block (it does so after the
 *  last character is deleted), so the placeholder shows and the `<br>` is not read as a newline. */
export function clearIfEmpty(field: HTMLElement): void {
	if (field.textContent === '' && field.childNodes.length > 0) {
		field.replaceChildren();
	}
}

/** Puts the nodes into an empty field. */
export function fillField(field: HTMLElement, nodes: readonly Node[]): void {
	for (const node of nodes) {
		field.appendChild(node);
	}
	syncSentinel(field);
}

/** The caret now, if it is inside `field`. */
export function caretIn(field: HTMLElement): Range | null {
	const selection = document.getSelection();
	if (selection === null || selection.rangeCount === 0) {
		return null;
	}
	const range = selection.getRangeAt(0);
	return field.contains(range.commonAncestorContainer) ? range.cloneRange() : null;
}

/** Puts `node` at `caret` (or at the end of the field when there is none), replacing what is
 *  selected, and the caret after it. Returns the new caret. */
export function insertAt(field: HTMLElement, caret: Range | null, node: Node): Range {
	let range = caret;
	if (range === null || !field.contains(range.commonAncestorContainer)) {
		range = document.createRange();
		range.selectNodeContents(field);
		range.collapse(false);
	}
	range.deleteContents();
	range.insertNode(node);
	range.setStartAfter(node);
	range.collapse(true);
	// `insertNode` splits a text node and can leave an empty one; normalizing joins the text nodes
	// (and moves this live range with them), so the field holds the fewest nodes.
	field.normalize();
	field.focus({ preventScroll: true });
	const selection = document.getSelection();
	selection?.removeAllRanges();
	selection?.addRange(range);
	return range.cloneRange();
}

/** Takes a chip out of the field. */
export function removeChip(chip: HTMLElement): void {
	chip.remove();
}
