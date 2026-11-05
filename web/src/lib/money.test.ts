import { describe, expect, it } from 'vitest';
import { amountText, exponentOf, formatMoney, parseAmount } from './money';

describe('exponentOf', () => {
	it('is 2 for colones', () => {
		expect(exponentOf('CRC', 'es-CR')).toBe(2);
	});

	it('is 0 for the yen', () => {
		expect(exponentOf('JPY', 'es-CR')).toBe(0);
	});

	it('is 2 for dollars', () => {
		expect(exponentOf('USD', 'es-CR')).toBe(2);
	});
});

describe('formatMoney', () => {
	it('writes whole units for a 0-decimal currency', () => {
		expect(formatMoney(30000, 'JPY', 'es-CR')).toContain('30');
	});

	it('divides by the minor unit', () => {
		expect(formatMoney(1250, 'USD', 'en-US')).toBe('$12.50');
	});
});

describe('parseAmount', () => {
	it('reads whole units for a 0-decimal currency', () => {
		expect(parseAmount('30000', 0)).toBe(30000);
	});

	it('reads a decimal comma', () => {
		expect(parseAmount('12,5', 2)).toBe(1250);
	});

	it('reads a decimal point', () => {
		expect(parseAmount('12.05', 2)).toBe(1205);
	});

	it('refuses text that is not an amount', () => {
		expect(parseAmount('doce', 2)).toBeNull();
	});

	it('refuses a negative amount', () => {
		expect(parseAmount('-5', 2)).toBeNull();
	});

	it('refuses more decimals than the currency has', () => {
		expect(parseAmount('1,555', 2)).toBeNull();
	});
});

describe('amountText', () => {
	it('writes the amount for the field', () => {
		expect(amountText(1250, 2)).toBe('12.50');
	});

	it('writes whole units for a 0-decimal currency', () => {
		expect(amountText(30000, 0)).toBe('30000');
	});
});
