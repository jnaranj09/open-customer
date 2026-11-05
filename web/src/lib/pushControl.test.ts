import { describe, expect, it } from 'vitest';
import type { Result } from './api/errors';
import { createPushControl } from './pushControl.svelte';
import type { PushState } from './push';

function setup(state: PushState, activated: Result<void>) {
	let release = (): void => undefined;
	let calls = 0;
	const control = createPushControl({
		check: () => Promise.resolve(state),
		activate: () => {
			calls += 1;
			return new Promise((resolve) => {
				release = () => {
					resolve(activated);
				};
			});
		}
	});
	return {
		control,
		release: (): void => {
			release();
		},
		calls: (): number => calls
	};
}

describe('the push control', () => {
	it('holds what the check said', async () => {
		const { control } = setup('needs_activation', { ok: true, data: undefined });
		await control.check();
		expect(control.state).toBe('needs_activation');
	});

	it('is active after a good activation', async () => {
		const { control, release } = setup('needs_activation', { ok: true, data: undefined });
		await control.check();
		const tap = control.activate();
		release();
		await tap;
		expect(control.state).toBe('active');
	});

	it('keeps needs_activation and says why after a refused activation', async () => {
		const { control, release } = setup('needs_activation', {
			ok: false,
			error: { kind: 'push_unavailable' }
		});
		await control.check();
		const tap = control.activate();
		release();
		await tap;
		expect([control.state, control.error]).toEqual([
			'needs_activation',
			'No se pudieron activar las notificaciones. Revisa el permiso en los ajustes del teléfono'
		]);
	});

	it('is busy while the tap runs and ignores a second tap', async () => {
		const { control, release, calls } = setup('needs_activation', { ok: true, data: undefined });
		const first = control.activate();
		const second = control.activate();
		expect([control.busy, calls()]).toEqual([true, 1]);
		release();
		await Promise.all([first, second]);
		expect(control.busy).toBe(false);
	});

	it('forgets the state on reset', async () => {
		const { control } = setup('active', { ok: true, data: undefined });
		await control.check();
		control.reset();
		expect(control.state).toBeNull();
	});
});
