import { describe, expect, it } from 'vitest';
import { t } from './index';

describe('t', () => {
	it('returns a plain message as it is', () => {
		expect(t('nav_today')).toBe('Hoy');
	});

	it('fills params', () => {
		expect(t('due_confirm_sent', { name: 'Ana' })).toBe('¿Lo enviaste a Ana?');
	});

	it('fills a numeric param', () => {
		expect(t('due_today_summary', { count: 2 })).toBe('2 mensajes para enviar hoy');
	});

	it('throws on a missing param', () => {
		expect(() => t('due_confirm_sent')).toThrow('name');
	});

	it('throws when only another param is given', () => {
		expect(() => t('due_confirm_sent', { other: 'x' })).toThrow('name');
	});
});
