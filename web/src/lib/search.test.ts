import { describe, expect, it } from 'vitest';
import { normalize, search, type Searchable } from './search';

const client = (id: string, name: string, phone_digits: string | null = null): Searchable => ({
	id,
	name,
	phone_digits
});

const ids = (items: Searchable[]): string[] => items.map((c) => c.id);

describe('normalize', () => {
	it('removes accents and lowercases', () => {
		expect(normalize('María ÁNA Ñu')).toBe('maria ana nu');
	});
});

describe('search', () => {
	it('matches a phone typed with a dash against the digits', () => {
		const items = [
			client('a', 'Ana Rojas', '50688887777'),
			client('b', 'Marta Solano', '50670001111')
		];
		expect(ids(search(items, '8888-7777'))).toEqual(['a']);
	});

	it('finds an accented name from an unaccented query', () => {
		expect(ids(search([client('a', 'María Vargas')], 'maria'))).toEqual(['a']);
	});

	it('finds a name from an accented query', () => {
		expect(ids(search([client('a', 'Maria Vargas')], 'MARÍA'))).toEqual(['a']);
	});

	it('ranks a word start before a match inside a word', () => {
		const items = [client('m', 'Mariana Solís'), client('a', 'Ána López')];
		expect(ids(search(items, 'ana'))).toEqual(['a', 'm']);
	});

	it('counts the start of a later word as a word start', () => {
		const items = [
			client('a', 'Lucía Rojas'),
			client('b', 'Mariana Solís'),
			client('c', 'Ana Rojas')
		];
		expect(ids(search(items, 'ana'))).toEqual(['c', 'b']);
	});

	it('orders each group by the accent-free name, then the raw name', () => {
		const items = [client('3', 'Ana Z'), client('2', 'Ána B'), client('1', 'Ana B')];
		expect(ids(search(items, 'ana'))).toEqual(['1', '2', '3']);
	});

	it('matches no phone from a query with letters only', () => {
		expect(search([client('a', 'Ana Rojas', '50688887777')], 'abc')).toEqual([]);
	});

	it('matches a phone substring from 3 digits', () => {
		expect(ids(search([client('a', 'Ana Rojas', '50688887777')], '888'))).toEqual(['a']);
	});

	it('does not match a phone from 2 digits', () => {
		expect(search([client('a', 'Ana Rojas', '50688887777')], '88')).toEqual([]);
	});

	it('does not match a phone from 1 digit', () => {
		expect(search([client('a', 'Ana Rojas', '50688887777')], '8')).toEqual([]);
	});

	it('skips a client with no phone for a digit query', () => {
		expect(search([client('a', 'Ana Rojas', null)], '8888')).toEqual([]);
	});

	it('still matches a short digit query inside a name', () => {
		expect(ids(search([client('a', 'Local 42')], '42'))).toEqual(['a']);
	});

	it('treats % as a plain character', () => {
		const items = [client('a', 'Ana Rojas'), client('b', '100% Ana')];
		expect(ids(search(items, '%'))).toEqual(['b']);
	});

	it('treats _ as a plain character', () => {
		const items = [client('a', 'Ana Rojas'), client('b', 'Ana_Rojas')];
		expect(ids(search(items, '_'))).toEqual(['b']);
	});

	it('returns nothing for an empty or blank query', () => {
		expect(search([client('a', 'Ana Rojas')], '   ')).toEqual([]);
	});

	it('returns the top 20 only', () => {
		const items = Array.from({ length: 25 }, (_, i) =>
			client(String(i), `Ana ${String(i).padStart(2, '0')}`)
		);
		expect(ids(search(items, 'ana'))).toEqual(ids(items.slice(0, 20)));
	});

	it('does not change the input', () => {
		const items = [client('b', 'Ana B'), client('a', 'Ana A')];
		search(items, 'ana');
		expect(ids(items)).toEqual(['b', 'a']);
	});
});
