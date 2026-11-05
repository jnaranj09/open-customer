import { describe, expect, it } from 'vitest';
import { navigationAllowed, templateDirty } from './templateEdit';

const saved = { name: 'Recordatorio', body: 'Hola {{client.name}}' };

describe('templateDirty', () => {
	it('is clean for a saved template with its own name and body', () => {
		expect(templateDirty(saved, 'Recordatorio', saved.body)).toBe(false);
	});

	it('ignores spaces around the name, which save trims', () => {
		expect(templateDirty(saved, '  Recordatorio ', saved.body)).toBe(false);
	});

	it('is dirty when the name changed', () => {
		expect(templateDirty(saved, 'Otro', saved.body)).toBe(true);
	});

	it('is dirty when the body changed, even by a space (the body is stored as sent)', () => {
		expect(templateDirty(saved, 'Recordatorio', `${saved.body} `)).toBe(true);
	});

	it('is clean for a new template left empty', () => {
		expect(templateDirty(undefined, '  ', '')).toBe(false);
	});

	it('is dirty for a new template with a name', () => {
		expect(templateDirty(undefined, 'Nueva', '')).toBe(true);
	});

	it('is dirty for a new template with a body', () => {
		expect(templateDirty(undefined, '', 'Hola')).toBe(true);
	});
});

describe('navigationAllowed', () => {
	it('lets anything go when nothing is unsaved', () => {
		expect(navigationAllowed(null, false, 'a', 'b')).toBe(true);
	});

	it('asks when there are unsaved changes and no exemption (an update or a restore stays on the page)', () => {
		expect(navigationAllowed(null, true, 'a', 'b')).toBe(false);
	});

	it('lets a discard she chose go', () => {
		expect(navigationAllowed({ kind: 'discard' }, true, 'a', 'b')).toBe(true);
	});

	it('lets the navigation after a create go when nothing changed since the save', () => {
		expect(navigationAllowed({ kind: 'saved', name: 'a', body: 'b' }, true, ' a ', 'b')).toBe(true);
	});

	it('asks after a create when the body changed during the request', () => {
		expect(navigationAllowed({ kind: 'saved', name: 'a', body: 'b' }, true, 'a', 'bc')).toBe(false);
	});

	it('asks after a create when the name changed during the request', () => {
		expect(navigationAllowed({ kind: 'saved', name: 'a', body: 'b' }, true, 'ab', 'b')).toBe(false);
	});
});
