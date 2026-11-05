// The pure parts of Ajustes → Atributos (design doc item 77): the archive warning, the option
// order, and the drag target. The screens own the state and the calls.
import type { AttrOption, Attribute, Template } from './api/schemas';
import { templateUses } from './chips';
import type { MessageKey } from './i18n';
import { byPosition } from './order';

/** The names of the active templates that use the attribute, in the order given. The warning is
 *  information only; the archive goes ahead (design doc "Attribute values"). */
export function archiveWarning(
	attr: Pick<Attribute, 'scope' | 'key'>,
	templates: readonly Template[]
): string[] {
	return templates
		.filter((x) => !x.archived && templateUses(x.body, attr.scope, attr.key))
		.map((x) => x.name);
}

export type Positioned = { id: string; position: number };

/** Moves the row at `from` to `to` among its siblings (sorted by position) and gives back the new
 *  position of each row that moved, ready to send as one `PATCH` each. The rows reuse the
 *  positions the siblings hold, so an archived sibling between them keeps its place; tied
 *  positions are numbered from 0. An index outside the list changes nothing. */
export function reorder(siblings: readonly Positioned[], from: number, to: number): Positioned[] {
	const moved = siblings[from];
	if (moved === undefined || to < 0 || to >= siblings.length) {
		return [];
	}
	const order = siblings.filter((_, i) => i !== from);
	order.splice(to, 0, moved);
	const held = siblings.map((s) => s.position);
	const distinct = held.every((p, i) => i === 0 || p > (held[i - 1] ?? -1));
	const slots = distinct ? held : held.map((_, i) => i);
	return order
		.map((s, i) => ({ id: s.id, position: slots[i] ?? i }))
		.filter((s) => siblings.find((old) => old.id === s.id)?.position !== s.position);
}

/** The active options under `parent` (null is the top level), in position order. */
export function levelOf(options: readonly AttrOption[], parent: string | null): AttrOption[] {
	return options
		.filter((o) => !o.archived && o.parent_id === parent)
		.sort((a, b) => a.position - b.position);
}

/** The row a dragged handle is over: the one whose middle is nearest to `y` (the first on a tie).
 *  `mids` are the rows' vertical middles in order. */
export function indexAtY(mids: readonly number[], y: number): number {
	let at = 0;
	mids.forEach((mid, i) => {
		if (Math.abs(mid - y) < Math.abs((mids[at] ?? mid) - y)) {
			at = i;
		}
	});
	return at;
}

/** The label of each attribute type, as she reads it. */
export const TYPE_LABELS: Record<Attribute['type'], MessageKey> = {
	text: 'attribute_type_text',
	number: 'attribute_type_number',
	date: 'attribute_type_date',
	select: 'attribute_type_select',
	multiselect: 'attribute_type_multiselect'
};

/** The short label of each scope: the heading of its group in the list. */
export const SCOPE_LABELS: Record<Attribute['scope'], MessageKey> = {
	client: 'attribute_scope_client_short',
	appointment: 'attribute_scope_appointment_short'
};

/** True for the types that take options. */
export function hasOptions(type: Attribute['type']): boolean {
	return type === 'select' || type === 'multiselect';
}

/** The attributes of one scope, in position order. */
export function inScope(
	attributes: readonly Attribute[],
	scope: Attribute['scope'],
	archived: boolean
): Attribute[] {
	return byPosition(
		attributes.filter((a) => a.scope === scope && a.archived === archived),
		(a) => a.label
	);
}
