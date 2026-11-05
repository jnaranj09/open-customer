import { describe, expect, it } from 'vitest';
import type { AttrOption, Attribute, Template } from './api/schemas';
import { archiveWarning, hasOptions, indexAtY, inScope, levelOf, reorder } from './attributes';

const ID = '3f2b8c1e-5d4a-4e6b-9c7d-1a2b3c4d5e6f';

function template(name: string, body: string, archived = false): Template {
	return { id: ID, name, body, archived };
}

function option(id: string, over: Partial<AttrOption> = {}): AttrOption {
	return {
		id,
		attribute_id: ID,
		parent_id: null,
		label: id,
		position: 0,
		archived: false,
		...over
	};
}

describe('archiveWarning', () => {
	const drink = { scope: 'client', key: 'bebida_favorita' } as const;

	it('lists the templates that use the attribute', () => {
		const templates = [
			template('Recordatorio', 'Hola {{client.first_name}}, ¿tu {{client.bebida_favorita}}?'),
			template('Relleno', 'Hola {{client.first_name}}'),
			template('Con respaldo', '{{ client.bebida_favorita | "algo rico" }}')
		];
		expect(archiveWarning(drink, templates)).toEqual(['Recordatorio', 'Con respaldo']);
	});

	it('ignores a template of the other scope with the same key', () => {
		const templates = [template('Cita', '{{appointment.bebida_favorita}}')];
		expect(archiveWarning(drink, templates)).toEqual([]);
	});

	it('ignores an archived template', () => {
		const templates = [template('Vieja', '{{client.bebida_favorita}}', true)];
		expect(archiveWarning(drink, templates)).toEqual([]);
	});
});

describe('reorder', () => {
	const rows = [
		{ id: 'a', position: 0 },
		{ id: 'b', position: 1 },
		{ id: 'c', position: 2 }
	];

	it('moves an option and returns the new positions of the rows that moved', () => {
		expect(reorder(rows, 2, 0)).toEqual([
			{ id: 'c', position: 0 },
			{ id: 'a', position: 1 },
			{ id: 'b', position: 2 }
		]);
	});

	it('returns only the two rows of a one-step move', () => {
		expect(reorder(rows, 0, 1)).toEqual([
			{ id: 'b', position: 0 },
			{ id: 'a', position: 1 }
		]);
	});

	it('returns nothing when the row stays', () => {
		expect(reorder(rows, 1, 1)).toEqual([]);
	});

	it('returns nothing for an index outside the list', () => {
		expect(reorder(rows, 0, 3)).toEqual([]);
	});

	it('reuses the positions the siblings hold, gaps included', () => {
		const gapped = [
			{ id: 'a', position: 0 },
			{ id: 'x', position: 3 },
			{ id: 'b', position: 7 }
		];
		expect(reorder(gapped, 2, 0)).toEqual([
			{ id: 'b', position: 0 },
			{ id: 'a', position: 3 },
			{ id: 'x', position: 7 }
		]);
	});

	it('numbers the rows from 0 when positions are tied', () => {
		const tied = [
			{ id: 'a', position: 0 },
			{ id: 'b', position: 0 }
		];
		expect(reorder(tied, 1, 0)).toEqual([{ id: 'a', position: 1 }]);
	});
});

describe('levelOf', () => {
	const options = [
		option('late', { position: 2 }),
		option('early', { position: 1 }),
		option('child', { parent_id: 'early' }),
		option('gone', { position: 0, archived: true })
	];

	it('lists the active options under a parent in position order', () => {
		expect(levelOf(options, null).map((o) => o.id)).toEqual(['early', 'late']);
	});

	it('lists the children of an option', () => {
		expect(levelOf(options, 'early').map((o) => o.id)).toEqual(['child']);
	});
});

describe('indexAtY', () => {
	const mids = [10, 30, 50];

	it('is the row whose middle is nearest to the pointer', () => {
		expect([indexAtY(mids, 19), indexAtY(mids, 21), indexAtY(mids, 44)]).toEqual([0, 1, 2]);
	});

	it('is 0 above the first row', () => {
		expect(indexAtY(mids, -5)).toBe(0);
	});

	it('is the last row below the last middle', () => {
		expect(indexAtY(mids, 500)).toBe(2);
	});
});

describe('inScope', () => {
	const base: Pick<Attribute, 'key' | 'label' | 'type' | 'required' | 'options'> = {
		key: 'k',
		label: 'x',
		type: 'text',
		required: false,
		options: []
	};
	const attrs: Attribute[] = [
		{ ...base, id: 'b', scope: 'client', position: 1, archived: false },
		{ ...base, id: 'a', scope: 'client', position: 0, archived: false },
		{ ...base, id: 'c', scope: 'appointment', position: 0, archived: false },
		{ ...base, id: 'd', scope: 'client', position: 2, archived: true }
	];

	it('lists the active attributes of a scope in position order', () => {
		expect(inScope(attrs, 'client', false).map((a) => a.id)).toEqual(['a', 'b']);
	});

	it('lists the archived ones apart', () => {
		expect(inScope(attrs, 'client', true).map((a) => a.id)).toEqual(['d']);
	});
});

describe('hasOptions', () => {
	it('is true for the two select types only', () => {
		expect(
			['text', 'number', 'date', 'select', 'multiselect'].map((x) =>
				hasOptions(x as Attribute['type'])
			)
		).toEqual([false, false, false, true, true]);
	});
});
