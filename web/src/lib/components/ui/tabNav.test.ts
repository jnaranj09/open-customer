import { describe, expect, it } from 'vitest';
import { isCurrent, type Tab } from './tabNav';

describe('isCurrent', () => {
	const exact: Tab = { href: '/enviar', label: 'tab_reminders', match: 'exact' };
	const prefix: Tab = { href: '/ajustes/plantillas', label: 'tab_templates', match: 'prefix' };

	it.each([
		['an exact tab on its own path', exact, '/enviar', true],
		['an exact tab on a path under it', exact, '/enviar/respuestas', false],
		['a prefix tab on its own path', prefix, '/ajustes/plantillas', true],
		['a prefix tab on a path under it', prefix, '/ajustes/plantillas/nueva', true],
		['a prefix tab on a longer name', prefix, '/ajustes/plantillasx', false]
	])('%s is %s', (_, tab, path, want) => {
		expect(isCurrent(tab, path)).toBe(want);
	});
});
