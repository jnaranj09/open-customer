// The draft lifecycle the three draft forms share (Nueva cita, the new-client form and the quote;
// design doc item 67): load a saved draft when the form opens, then, once it is loaded, save the
// form 500 ms after each change that differs from the start and clear the draft when the form is
// back at its start (`draftStep`).
import { onMount, untrack } from 'svelte';
import type { z } from 'zod';
import { autosave, draftStep } from './drafts';

/** Call while a component initialises. `key` is null for a form that keeps no draft (an edit).
 *  `get` reads a plain copy of the form's values (`$state.snapshot(form)`), `set` puts a restored draft into the form. */
export function formDraft<T>(
	key: string | null,
	schema: z.ZodType<T>,
	initial: T,
	get: () => T,
	set: (value: T) => void
) {
	const store = key === null ? null : autosave(key, schema);
	let restored = $state(false);
	/** True once the saved draft has been read (and put into the form). Stays false with no key. */
	let loaded = $state(false);

	onMount(() => {
		if (store === null) {
			return;
		}
		void store
			.load()
			.then((saved) => {
				if (saved !== null) {
					set(saved);
					restored = true;
				}
			})
			.finally(() => {
				loaded = true;
			});
	});

	$effect(() => {
		const now = get();
		const step = draftStep(loaded, now, initial);
		untrack(() => {
			if (step === 'save') {
				store?.save(now);
			} else if (step === 'clear') {
				void clear();
			}
		});
	});

	/** Forgets the draft; a failed delete is logged, never thrown. */
	async function clear(): Promise<void> {
		try {
			await store?.clear();
		} catch {
			console.warn('code=draft_clear_failed');
		}
	}

	return {
		/** True while the form shows a recovered draft. */
		get restored(): boolean {
			return restored;
		},
		clear,
		/** "Empezar de nuevo": the form goes back to its start and the draft goes. */
		async startOver(): Promise<void> {
			restored = false;
			set(structuredClone(initial));
			await clear();
		}
	};
}
