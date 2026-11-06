// The known findings of `ui-probe.spec.ts`, each waiting on a fix. `bug` is the finding's id
// (`UIP-` and the id it had when it was found: M and L from the release-1 bug hunt, N from this
// probe) and `why` says what the bug is. Only shrinks: an entry that matches nothing on the screen
// it is scoped to fails, so a fix removes its entries in the same change. A new entry needs the
// owner's yes.
import type { Allow, Width } from './probe';

type Common = { width?: Width; target?: string; why: string; bug: string };

/** One entry per screen state of `route` that shows the finding. */
function entries(route: string, check: string, states: string[], common: Common): Allow[] {
	return states.map((state) => ({ route, check, state, ...common }));
}

export const ALLOW: readonly Allow[] = [
	// /
	...entries('/', 'empty', ['agenda empty'], {
		width: 'wide',
		why: 'the wide week view says nothing for an empty week',
		bug: 'UIP-N2'
	}),
	// /ajustes/plantillas/[id]
	...entries('/ajustes/plantillas/[id]', 'busy', ['template busy'], {
		width: 'compact',
		why: 'the compact template editor is blank while the templates load',
		bug: 'UIP-N5'
	}),
	// /ajustes/plantillas/nueva
	...entries('/ajustes/plantillas/nueva', 'busy', ['form data busy'], {
		width: 'compact',
		why: 'the compact template editor is blank while the templates load',
		bug: 'UIP-N5'
	}),
	// /citas/[id]
	...entries('/citas/[id]', 'busy', ['photos busy'], {
		why: 'the photo section shows no loading line while the photos load',
		bug: 'UIP-N4'
	}),
	// /clientas/[id]
	...entries('/clientas/[id]', 'busy', ['reminder log busy'], {
		why: 'the reminder log shows no loading line while it loads',
		bug: 'UIP-L11'
	}),
	...entries('/clientas/[id]', 'busy', ['photos busy'], {
		why: 'the photo section shows no loading line while the photos load',
		bug: 'UIP-N4'
	}),
	// /enviar
	...entries('/enviar', 'busy', ['sent log busy'], {
		why: 'the reminder log shows no loading line while it loads',
		bug: 'UIP-L11'
	}),
	...entries('/enviar', 'busy', ['due list busy'], {
		why: 'Enviar shows no loading line while the due list loads',
		bug: 'UIP-N3'
	})
];
