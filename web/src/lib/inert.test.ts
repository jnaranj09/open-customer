import { describe, expect, it } from 'vitest';
import { LIVE_LAYER, inertBehind, type Layer } from './inert';

function layer(attrs: string[] = [], inert = false): Layer {
	return { inert, hasAttribute: (name) => attrs.includes(name) };
}

describe('inertBehind', () => {
	it('makes the page inert and leaves the top sheet live', () => {
		const [app, sheet] = [layer(), layer()];
		inertBehind([app, sheet], sheet, new Set());
		expect([app.inert, sheet.inert]).toEqual([true, false]);
	});

	it('makes a lower sheet inert under a second one and frees it when the top closes', () => {
		const [app, lower, upper] = [layer(), layer(), layer()];
		const held = new Set<Layer>();
		inertBehind([app, lower], lower, held);
		inertBehind([app, lower, upper], upper, held);
		expect(lower.inert).toBe(true);
		inertBehind([app, lower], lower, held);
		expect([app.inert, lower.inert]).toEqual([true, false]);
	});

	it('frees the page when the last sheet closes', () => {
		const [app, sheet] = [layer(), layer()];
		const held = new Set<Layer>();
		inertBehind([app, sheet], sheet, held);
		inertBehind([app], null, held);
		expect([app.inert, held.size]).toEqual([false, 0]);
	});

	it('leaves a live layer (the toast) tappable', () => {
		const [app, toast, sheet] = [layer(), layer([LIVE_LAYER]), layer()];
		inertBehind([app, toast, sheet], sheet, new Set());
		expect([app.inert, toast.inert]).toEqual([true, false]);
	});

	it('does not free a layer that was inert before the sheet opened', () => {
		const [app, sheet] = [layer([], true), layer()];
		const held = new Set<Layer>();
		inertBehind([app, sheet], sheet, held);
		inertBehind([app], null, held);
		expect(app.inert).toBe(true);
	});
});
