// The draft lifecycle both forms share (design doc item 67): load a saved draft when the form
// opens, save the form 500 ms after each change that differs from the start, and clear it.
import { onMount, untrack } from 'svelte';
import type { z } from 'zod';
import { autosave } from './drafts';

/** Call while a component initialises. `key` is null for a form that keeps no draft (an edit).
 *  `get` reads a plain copy of the form's values (`$state.snapshot(form)`), `set` puts a restored draft into the form. */
export function formDraft<T>(
	key: string | null,
	schema: z.ZodType<T>,
	initial: T,
	get: () => T,
	set: (value: T) => void
) {
	const draft = key === null ? null : autosave(key, schema);
	let restored = $state(false);

	onMount(() => {
		void draft?.load().then((saved) => {
			if (saved !== null) {
				set(saved);
				restored = true;
			}
		});
	});

	$effect(() => {
		const now = get();
		if (JSON.stringify(now) !== JSON.stringify(initial)) {
			untrack(() => {
				draft?.save(now);
			});
		}
	});

	/** Forgets the draft; a failed delete is logged, never thrown. */
	async function clear(): Promise<void> {
		try {
			await draft?.clear();
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
