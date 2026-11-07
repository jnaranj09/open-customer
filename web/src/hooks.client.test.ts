import { describe, expect, it } from 'vitest';
import { z } from 'zod';
import { init } from './hooks.client';

describe('client init', () => {
	it('turns off zod parsers compiled with new Function, which the CSP forbids', () => {
		init();
		expect(z.config().jitless).toBe(true);
	});
});
