// The push state every screen with an "Activar notificaciones" button reads: what `checkPush` said,
// whether the tap is running, and why it failed. The shell checks on open; the sidebar and Hoy show
// the button from this one owner.
import { api } from './api/app';
import { errorText, type Result } from './api/errors';
import { activatePush, checkPush, type PushState } from './push';
import { browserPushDeps } from './push-browser';
import { submitOnce } from './submit';

type Deps = {
	check: () => Promise<PushState>;
	activate: () => Promise<Result<void>>;
};

export function createPushControl(deps: Deps) {
	let state = $state<PushState | null>(null);
	let busy = $state(false);
	let error = $state<string | null>(null);

	const tap = submitOnce(
		async () => {
			error = null;
			const result = await deps.activate();
			if (result.ok) {
				state = 'active';
			} else {
				error = errorText(result.error);
			}
		},
		(pending) => {
			busy = pending;
		}
	);

	return {
		/** Null until the first check, and after `reset`. */
		get state(): PushState | null {
			return state;
		},
		get busy(): boolean {
			return busy;
		},
		get error(): string | null {
			return error;
		},
		/** Never asks for permission (see `checkPush`). */
		async check(): Promise<void> {
			state = await deps.check();
		},
		/** The "Activar notificaciones" tap. */
		activate: async (): Promise<void> => {
			await tap.run();
		},
		reset(): void {
			state = null;
			error = null;
		}
	};
}

export const pushControl = createPushControl({
	check: () => checkPush(browserPushDeps(api)),
	activate: () => activatePush(browserPushDeps(api))
});
