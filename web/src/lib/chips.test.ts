import { describe, expect, it } from 'vitest';
import {
	bodyProblems,
	chipKnown,
	chipLabel,
	chipText,
	fallbackError,
	parseBody,
	piecesFromDom,
	serializeBody,
	templateUses,
	type Piece
} from './chips';

const roundTrip = (body: string): string => serializeBody(parseBody(body));

const name: Piece = { kind: 'var', ns: 'client', key: 'first_name', fallback: null };
const drink: Piece = { kind: 'var', ns: 'client', key: 'drink', fallback: 'algo rico' };

describe('chips round trip (serializeBody of parseBody)', () => {
	it('keeps plain text', () => {
		expect(roundTrip('Hola, te esperamos mañana')).toBe('Hola, te esperamos mañana');
	});

	it('keeps a variable', () => {
		expect(roundTrip('Hola {{client.first_name}}, nos vemos')).toBe(
			'Hola {{client.first_name}}, nos vemos'
		);
	});

	it('keeps a variable with a fallback', () => {
		expect(roundTrip('Tu {{client.drink | "algo rico"}} te espera')).toBe(
			'Tu {{client.drink | "algo rico"}} te espera'
		);
	});

	it('keeps accents and emoji', () => {
		expect(roundTrip('¡Aquí estás, Ána! 💅🏽 {{business.name}}')).toBe(
			'¡Aquí estás, Ána! 💅🏽 {{business.name}}'
		);
	});

	it('keeps line breaks', () => {
		expect(roundTrip('Hola\n\n{{appointment.date}}\n')).toBe('Hola\n\n{{appointment.date}}\n');
	});

	it('keeps a lone brace, a "}}" and a "{{" that starts no variable', () => {
		expect(roundTrip('a { b } c }} d {{ e {{client}}')).toBe('a { b } c }} d {{ e {{client}}');
	});

	it('writes a variable with extra spaces in the form the editor writes', () => {
		expect(roundTrip('{{  client.name  |  "x"  }}')).toBe('{{client.name | "x"}}');
	});
});

describe('parseBody', () => {
	it('splits text and variables in body order', () => {
		expect(parseBody('Hola {{client.first_name}}!')).toEqual([
			{ kind: 'text', text: 'Hola ' },
			name,
			{ kind: 'text', text: '!' }
		]);
	});

	it('reads the fallback text', () => {
		expect(parseBody('{{client.drink | "algo rico"}}')).toEqual([drink]);
	});

	it('reads an empty fallback as an empty string, not as none', () => {
		expect(parseBody('{{client.drink | ""}}')).toEqual([{ ...drink, fallback: '' }]);
	});

	it('reads a fallback with accents, emoji and a line break', () => {
		expect(parseBody('{{client.drink | "café ☕\nfrío"}}')).toEqual([
			{ ...drink, fallback: 'café ☕\nfrío' }
		]);
	});

	it('keeps the spaces inside a fallback', () => {
		expect(parseBody('{{client.drink | " x "}}')).toEqual([{ ...drink, fallback: ' x ' }]);
	});

	it('accepts digits and underscores in a key after its first letter', () => {
		expect(parseBody('{{appointment.shape_2}}')).toEqual([
			{ kind: 'var', ns: 'appointment', key: 'shape_2', fallback: null }
		]);
	});

	it('keeps a token with a key starting in a digit as text', () => {
		expect(parseBody('{{client.2x}}')).toEqual([{ kind: 'text', text: '{{client.2x}}' }]);
	});

	it('keeps a token with a curly quote as text', () => {
		expect(parseBody('{{client.drink | “x”}}')).toEqual([
			{ kind: 'text', text: '{{client.drink | “x”}}' }
		]);
	});

	it('keeps a token with a non-ASCII space as text', () => {
		expect(parseBody('{{client.name }}')).toEqual([{ kind: 'text', text: '{{client.name }}' }]);
	});

	it('keeps an unclosed fallback as text', () => {
		expect(parseBody('{{client.drink | "x}}')).toEqual([
			{ kind: 'text', text: '{{client.drink | "x}}' }
		]);
	});

	it('keeps a variable of an unknown namespace as text, for the server to refuse', () => {
		expect(parseBody('{{other.name}}')).toEqual([{ kind: 'text', text: '{{other.name}}' }]);
	});

	it('does not read a variable inside the fallback of a token it keeps as text', () => {
		expect(parseBody('{{other.x | "{{client.first_name}}"}}')).toEqual([
			{ kind: 'text', text: '{{other.x | "{{client.first_name}}"}}' }
		]);
	});

	it('finds a variable right after a "{{" that starts no variable', () => {
		expect(parseBody('{{{{client.first_name}}')).toEqual([{ kind: 'text', text: '{{' }, name]);
	});

	it('gives no pieces for an empty body', () => {
		expect(parseBody('')).toEqual([]);
	});
});

describe('serializeBody', () => {
	it('writes the fallback with its straight quotes', () => {
		expect(serializeBody([drink])).toBe('{{client.drink | "algo rico"}}');
	});

	it('writes an empty fallback', () => {
		expect(serializeBody([{ ...drink, fallback: '' }])).toBe('{{client.drink | ""}}');
	});
});

describe('templateUses', () => {
	it('finds a variable without a fallback', () => {
		expect(templateUses('Hola {{client.drink}}', 'client', 'drink')).toBe(true);
	});

	it('finds a variable with a fallback', () => {
		expect(templateUses('Hola {{client.drink | "algo"}}', 'client', 'drink')).toBe(true);
	});

	it('ignores the same key in another namespace', () => {
		expect(templateUses('Hola {{appointment.drink}}', 'client', 'drink')).toBe(false);
	});

	it('ignores another key', () => {
		expect(templateUses('Hola {{client.drink_2}}', 'client', 'drink')).toBe(false);
	});

	it('ignores a token that is not a valid variable', () => {
		expect(templateUses('Hola {{client.drink', 'client', 'drink')).toBe(false);
	});
});

describe('fallbackError', () => {
	it('refuses a straight quote', () => {
		expect(fallbackError('el "de siempre"')).toBe('quote');
	});

	it('accepts text with accents, emoji and curly quotes', () => {
		expect(fallbackError('café ☕ “frío”')).toBeNull();
	});
});

describe('piecesFromDom', () => {
	it('joins the text nodes around a chip', () => {
		expect(
			piecesFromDom([
				{ kind: 'text', text: 'Hola ' },
				{ kind: 'text', text: 'Ana ' },
				{ kind: 'chip', ns: 'client', key: 'first_name', fallback: null },
				{ kind: 'text', text: '!' }
			])
		).toEqual([{ kind: 'text', text: 'Hola Ana ' }, name, { kind: 'text', text: '!' }]);
	});

	it('reads a line break as a newline', () => {
		expect(
			piecesFromDom([{ kind: 'text', text: 'a' }, { kind: 'br' }, { kind: 'text', text: 'b' }])
		).toEqual([{ kind: 'text', text: 'a\nb' }]);
	});

	it('puts a newline between the lines a browser wraps in blocks', () => {
		expect(
			piecesFromDom([
				{ kind: 'block', children: [{ kind: 'text', text: 'a' }] },
				{ kind: 'block', children: [{ kind: 'br' }] },
				{ kind: 'block', children: [{ kind: 'text', text: 'b' }] }
			])
		).toEqual([{ kind: 'text', text: 'a\n\nb' }]);
	});

	it('keeps the fallback of a chip', () => {
		expect(
			piecesFromDom([{ kind: 'chip', ns: 'client', key: 'drink', fallback: 'algo rico' }])
		).toEqual([drink]);
	});

	it('drops a chip whose namespace is not one the editor writes', () => {
		expect(
			piecesFromDom([
				{ kind: 'text', text: 'a' },
				{ kind: 'chip', ns: 'other', key: 'x', fallback: null }
			])
		).toEqual([{ kind: 'text', text: 'a' }]);
	});

	it('gives no pieces for an empty editor', () => {
		expect(piecesFromDom([])).toEqual([]);
	});
});

describe('chipLabel', () => {
	const vars = [{ ns: 'client', key: 'drink', label: 'Bebida favorita' }];
	const attributes = [{ scope: 'client' as const, key: 'old', label: 'Color anterior' }];

	it('uses the label the server offers', () => {
		expect(chipLabel('client', 'drink', vars, attributes)).toBe('Bebida favorita');
	});

	it('uses the label of an archived attribute the chip row no longer offers', () => {
		expect(chipLabel('client', 'old', vars, attributes)).toBe('Color anterior');
	});

	it('does not use an attribute of the other scope', () => {
		expect(chipLabel('appointment', 'old', vars, attributes)).toBe('old');
	});
});

describe('chipText', () => {
	it('shows only the label without a fallback', () => {
		expect(chipText('Bebida favorita', null)).toBe('Bebida favorita');
	});

	it('shows only the label for an empty fallback', () => {
		expect(chipText('Bebida favorita', '')).toBe('Bebida favorita');
	});

	it('shows the fallback after the label', () => {
		expect(chipText('Bebida favorita', 'algo rico')).toBe('Bebida favorita · algo rico');
	});
});

describe('bodyProblems', () => {
	const known = (_ns: string, key: string): boolean => key !== 'gone';
	const text = (t: string): Piece => ({ kind: 'text', text: t });

	it('finds nothing in text and known variables', () => {
		expect(bodyProblems([text('Hola '), name, text(' { y } }}')], known)).toEqual([]);
	});

	it('finds a "{{" typed in the text', () => {
		expect(bodyProblems([text('Hola {{client.name}}')], known)).toEqual(['braces']);
	});

	it('finds a "{" right before a variable, which the saved body would read as a "{{"', () => {
		expect(bodyProblems([text('Hola {'), name], known)).toEqual(['braces']);
	});

	it('does not mind a "{" before a variable when a space is between them', () => {
		expect(bodyProblems([text('Hola { '), name], known)).toEqual([]);
	});

	it('finds a variable that no longer exists', () => {
		expect(
			bodyProblems([{ kind: 'var', ns: 'client', key: 'gone', fallback: null }], known)
		).toEqual(['unknown']);
	});

	it('names each problem once, in the order first found', () => {
		expect(
			bodyProblems(
				[
					{ kind: 'var', ns: 'client', key: 'gone', fallback: null },
					text('{{'),
					{ kind: 'var', ns: 'client', key: 'gone', fallback: null },
					text('{{')
				],
				known
			)
		).toEqual(['unknown', 'braces']);
	});
});

describe('chipKnown', () => {
	const vars = [{ ns: 'client', key: 'drink', label: 'Bebida favorita' }];
	const attributes = [{ scope: 'client' as const, key: 'old', label: 'Color anterior' }];

	it('knows a variable the row offers', () => {
		expect(chipKnown('client', 'drink', vars, attributes)).toBe(true);
	});

	it('knows an archived attribute', () => {
		expect(chipKnown('client', 'old', vars, attributes)).toBe(true);
	});

	it('does not know another key, nor an attribute of the other scope', () => {
		expect([
			chipKnown('client', 'nope', vars, attributes),
			chipKnown('appointment', 'old', vars, attributes)
		]).toEqual([false, false]);
	});
});
