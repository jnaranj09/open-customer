import { describe, expect, it } from 'vitest';
import type { Settings } from './api/schemas';
import { businessPatch, showSettingsBack } from './settingsPage';

const PINK = ['#', 'c76b85'].join('');
const GREEN = ['#', '2e7d5b'].join('');

const saved: Settings = {
	name: 'Estudio Ana',
	timezone: 'America/Costa_Rica',
	locale: 'es',
	currency: 'CRC',
	country_code: 'CR',
	accent_color: PINK,
	push_hour: 8,
	has_custom_icon: false
};
const same = { name: 'Estudio Ana', timezone: 'America/Costa_Rica', color: PINK, pushHour: '8' };

describe('showSettingsBack', () => {
	it('is off on the Ajustes root', () => {
		expect([showSettingsBack('/ajustes'), showSettingsBack('/ajustes/')]).toEqual([false, false]);
	});

	it('is on for a section', () => {
		expect(showSettingsBack('/ajustes/atributos')).toBe(true);
	});

	it('is on for the template list', () => {
		expect(showSettingsBack('/ajustes/plantillas')).toBe(true);
	});

	it('is off in a template editor, which has its own back', () => {
		expect([
			showSettingsBack('/ajustes/plantillas/nueva'),
			showSettingsBack('/ajustes/plantillas/x')
		]).toEqual([false, false]);
	});
});

describe('businessPatch', () => {
	it('is empty when nothing changed', () => {
		expect(businessPatch(saved, same)).toEqual({});
	});

	it('holds a trimmed name', () => {
		expect(businessPatch(saved, { ...same, name: ' Estudio Luz ' })).toEqual({
			name: 'Estudio Luz'
		});
	});

	it('holds the new color and the new hour as a number', () => {
		expect(businessPatch(saved, { ...same, color: GREEN, pushHour: '10' })).toEqual({
			accent_color: GREEN,
			push_hour: 10
		});
	});

	it('does not count the case of a color as a change', () => {
		expect(businessPatch(saved, { ...same, color: PINK.toUpperCase() })).toEqual({});
	});
});
