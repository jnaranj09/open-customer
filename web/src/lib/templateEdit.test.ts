import { describe, expect, it } from 'vitest';
import { templateDirty } from './templateEdit';

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
