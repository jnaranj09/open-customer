import { describe, expect, it } from 'vitest';
import type { QuickReply } from './api/schemas';
import { filterReplies } from './replies';

const reply = (title: string, body: string, position: number): QuickReply => ({
	id: `${String(position)}0000000-0000-4000-8000-000000000000`,
	title,
	body,
	position,
	archived: false
});

const LIST = [
	reply('Dirección', 'Calle 1, San José', 0),
	reply('Precios', 'Uñas acrílicas: 20 USD', 1),
	reply('Horario', 'Abro de 9 a 6', 2)
];

describe('filterReplies', () => {
	it('keeps every reply, in her order, for a blank query', () => {
		expect(filterReplies(LIST, '  ').map((r) => r.title)).toEqual([
			'Dirección',
			'Precios',
			'Horario'
		]);
	});

	it('finds a title without its accent and in another case', () => {
		expect(filterReplies(LIST, 'DIRECCION').map((r) => r.title)).toEqual(['Dirección']);
	});

	it('finds text in the body', () => {
		expect(filterReplies(LIST, 'unas').map((r) => r.title)).toEqual(['Precios']);
	});

	it('keeps her order among several matches', () => {
		expect(filterReplies(LIST, 'a').map((r) => r.title)).toEqual([
			'Dirección',
			'Precios',
			'Horario'
		]);
	});

	it('finds nothing for a query no reply holds', () => {
		expect(filterReplies(LIST, 'zz')).toEqual([]);
	});
});
