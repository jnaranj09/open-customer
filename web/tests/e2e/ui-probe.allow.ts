// The known findings of `ui-probe.spec.ts`, each waiting on a fix. `bug` is the finding's id
// (`UIP-` and the id it had when it was found: M and L from the release-1 bug hunt, N from this
// probe) and `why` says what the bug is. Only shrinks: an entry that matches nothing on the screen
// it is scoped to fails, so a fix removes its entries in the same change. A new entry needs the
// owner's yes.
import { LONG_NAME, type Allow, type Width } from './probe';

type Common = { width?: Width; target?: string; why: string; bug: string };

/** One entry per screen state of `route` that shows the finding. */
function entries(route: string, check: string, states: string[], common: Common): Allow[] {
	return states.map((state) => ({ route, check, state, ...common }));
}

export const ALLOW: readonly Allow[] = [
	// /
	...entries('/', 'axe:scrollable-region-focusable', ['agenda empty'], {
		width: 'compact',
		why: 'the empty day grid scrolls but holds nothing focusable',
		bug: 'UIP-N7'
	}),
	...entries('/', 'clip', ['base'], {
		target: `a "10:00 ${LONG_NAME} Acrílico"`,
		why: 'a 30-minute appointment box in the day grid cuts its three lines',
		bug: 'UIP-M1'
	}),
	...entries('/', 'empty', ['agenda empty'], {
		width: 'wide',
		why: 'the wide week view says nothing for an empty week',
		bug: 'UIP-N2'
	}),
	...entries('/', 'target-size', ['base'], {
		target: `a "10:00 ${LONG_NAME} Acrílico"`,
		why: 'a 30-minute appointment box in the day grid is 28 px tall',
		bug: 'UIP-M1'
	}),
	// /ajustes/atributos
	...entries('/ajustes/atributos', 'inert', ['new attribute'], {
		why: 'the page behind an open BottomSheet stays reachable (not inert)',
		bug: 'UIP-L16'
	}),
	// /ajustes/plantillas/[id]
	...entries('/ajustes/plantillas/[id]', 'busy', ['template busy'], {
		width: 'compact',
		why: 'the compact template editor is blank while the templates load',
		bug: 'UIP-N5'
	}),
	...entries('/ajustes/plantillas/[id]', 'target-size', ['base'], {
		target: 'span "Bebida favorita. Toca para cambiar lo que se escribe si no tiene el dato"',
		why: 'a variable chip in the message editor is 26 px tall',
		bug: 'UIP-N6'
	}),
	...entries('/ajustes/plantillas/[id]', 'target-size', ['base'], {
		target: 'span "name. Toca para cambiar lo que se escribe si no tiene el dato"',
		why: 'a variable chip in the message editor is 26 px tall',
		bug: 'UIP-N6'
	}),
	// /ajustes/plantillas/nueva
	...entries('/ajustes/plantillas/nueva', 'busy', ['form data busy'], {
		width: 'compact',
		why: 'the compact template editor is blank while the templates load',
		bug: 'UIP-N5'
	}),
	// /ajustes/recordatorios
	...entries('/ajustes/recordatorios', 'inert', ['new rule'], {
		why: 'the page behind an open BottomSheet stays reachable (not inert)',
		bug: 'UIP-L16'
	}),
	// /ajustes/servicios
	...entries('/ajustes/servicios', 'inert', ['new service'], {
		why: 'the page behind an open BottomSheet stays reachable (not inert)',
		bug: 'UIP-L16'
	}),
	// /citas/[id]
	...entries('/citas/[id]', 'busy', ['photos busy'], {
		why: 'the photo section shows no loading line while the photos load',
		bug: 'UIP-N4'
	}),
	...entries('/citas/[id]', 'scroll', ['base', 'photos empty', 'photos error'], {
		width: 'compact',
		why: 'a one-word 60-letter name pushes the page wider than the screen',
		bug: 'UIP-M5'
	}),
	...entries('/citas/[id]', 'text-overflow', ['base', 'photos empty', 'photos error'], {
		target: `h2 "${LONG_NAME}"`,
		why: 'a one-word 60-letter name does not wrap and spills out of its box',
		bug: 'UIP-M5'
	}),
	// /citas/nueva
	...entries('/citas/nueva', 'inert', ['quick-add client'], {
		why: 'the page behind an open BottomSheet stays reachable (not inert)',
		bug: 'UIP-L16'
	}),
	// /clientas
	...entries('/clientas', 'scroll', ['base'], {
		width: 'compact',
		why: 'a one-word 60-letter name pushes the page wider than the screen',
		bug: 'UIP-M5'
	}),
	...entries('/clientas', 'text-overflow', ['base'], {
		target: `span "${LONG_NAME}"`,
		why: 'a one-word 60-letter name does not wrap and spills out of its box',
		bug: 'UIP-M5'
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
	...entries('/clientas/[id]', 'inert', ['archive'], {
		why: 'the page behind an open BottomSheet stays reachable (not inert)',
		bug: 'UIP-L16'
	}),
	...entries(
		'/clientas/[id]',
		'scroll',
		['archive', 'base', 'photos empty', 'photos error', 'reminder log empty', 'reminder log error'],
		{ why: 'a one-word 60-letter name pushes the page wider than the screen', bug: 'UIP-M5' }
	),
	...entries(
		'/clientas/[id]',
		'target-size',
		['archive', 'base', 'photos empty', 'photos error', 'reminder log empty', 'reminder log error'],
		{
			width: 'compact',
			target: 'a "Volver"',
			why: 'the client page back button shrinks to 22 px beside a long name',
			bug: 'UIP-M6'
		}
	),
	...entries(
		'/clientas/[id]',
		'text-overflow',
		['archive', 'base', 'photos empty', 'photos error'],
		{
			width: 'compact',
			target: `span "Hola ${LONG_NAME}, te esperamos hoy."`,
			why: 'a one-word 60-letter name does not wrap and spills out of its box',
			bug: 'UIP-M5'
		}
	),
	...entries(
		'/clientas/[id]',
		'text-overflow',
		[
			'archive',
			'base',
			'client error',
			'photos empty',
			'photos error',
			'reminder log empty',
			'reminder log error'
		],
		{
			width: 'wide',
			target: `span "${LONG_NAME}"`,
			why: 'a one-word 60-letter name does not wrap and spills out of its box',
			bug: 'UIP-M5'
		}
	),
	// /clientas/nueva
	...entries('/clientas/nueva', 'text-overflow', ['base'], {
		width: 'wide',
		target: `span "${LONG_NAME}"`,
		why: 'a one-word 60-letter name does not wrap and spills out of its box',
		bug: 'UIP-M5'
	}),
	// /enviar
	...entries('/enviar', 'busy', ['sent log busy'], {
		why: 'the reminder log shows no loading line while it loads',
		bug: 'UIP-L11'
	}),
	...entries('/enviar', 'busy', ['due list busy'], {
		why: 'Enviar shows no loading line while the due list loads',
		bug: 'UIP-N3'
	}),
	...entries('/enviar', 'clip', ['base', 'due list empty', 'due list error'], {
		target: `span "Hola ${LONG_NAME}, te esperamos hoy."`,
		why: 'a message with a one-word 60-letter name is cut in its row',
		bug: 'UIP-M5'
	}),
	...entries('/enviar', 'clip', ['base', 'sent log empty', 'sent log error'], {
		width: 'wide',
		target: `span "Hola ${LONG_NAME}, te esperamos mañana."`,
		why: 'a message with a one-word 60-letter name is cut in its row',
		bug: 'UIP-M5'
	}),
	...entries('/enviar', 'scroll', ['base', 'sent log empty', 'sent log error'], {
		width: 'compact',
		why: 'a one-word 60-letter name pushes the page wider than the screen',
		bug: 'UIP-M5'
	}),
	...entries('/enviar', 'text-overflow', ['base', 'sent log empty', 'sent log error'], {
		width: 'compact',
		target: `h2 "${LONG_NAME}"`,
		why: 'a one-word 60-letter name does not wrap and spills out of its box',
		bug: 'UIP-M5'
	}),
	...entries('/enviar', 'text-overflow', ['base', 'sent log empty', 'sent log error'], {
		width: 'compact',
		target: `p "Hola ${LONG_NAME}, te esperamos mañana."`,
		why: 'a one-word 60-letter name does not wrap and spills out of its box',
		bug: 'UIP-M5'
	})
];
