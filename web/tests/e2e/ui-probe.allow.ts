// The known findings of `ui-probe.spec.ts`, each waiting on the fix its `bug` names (the id in the
// bug report the fix is tracked under; N1 to N7 were found by this probe). Only shrinks: an entry
// that matches nothing on the screen it is scoped to fails, so a fix removes its entries in the
// same change. A new entry needs the owner's yes.
import type { Allow, Width } from './probe';

/** The made-up 60-letter name starts with this. */
const NAME = 'Mariafernanda';

type Common = { width?: Width; target?: string; reason: string; bug: string };

/** One entry per screen state of `route` that shows the finding. */
function entries(route: string, check: string, states: string[], common: Common): Allow[] {
	return states.map((state) => ({ route, check, state, ...common }));
}

export const ALLOW: readonly Allow[] = [
	{
		route: '*',
		witness: '/',
		check: 'axe:document-title',
		reason: 'no <title> on any page',
		bug: 'L1'
	},
	{
		route: '*',
		witness: '/',
		check: 'axe:color-contrast',
		detail: 'foreground color: #ffffff, background color: #c76b85',
		reason: 'white on the default accent is 3.57:1, under 4.5:1',
		bug: 'N1'
	},
	// /
	...entries('/', 'axe:scrollable-region-focusable', ['agenda empty'], {
		width: 'compact',
		reason: 'the empty day grid scrolls but holds nothing focusable',
		bug: 'N7'
	}),
	...entries('/', 'clip', ['base'], {
		target: NAME,
		reason: 'a 30-minute appointment box cuts its three lines',
		bug: 'M1'
	}),
	...entries('/', 'empty', ['agenda empty'], {
		width: 'wide',
		reason: 'the week view says nothing for an empty week',
		bug: 'N2'
	}),
	...entries('/', 'target-size', ['base'], {
		target: NAME,
		reason: 'a 30-minute appointment box is 28 px tall',
		bug: 'M1'
	}),
	// /ajustes/atributos
	...entries('/ajustes/atributos', 'inert', ['new attribute'], {
		reason: 'the page behind an open BottomSheet is not inert',
		bug: 'L16'
	}),
	// /ajustes/plantillas/[id]
	...entries('/ajustes/plantillas/[id]', 'busy', ['template busy'], {
		width: 'compact',
		reason: 'the compact template editor shows nothing while the templates load',
		bug: 'N5'
	}),
	...entries('/ajustes/plantillas/[id]', 'target-size', ['base'], {
		target: 'Toca para cambiar',
		reason: 'a variable chip in the message editor is 26 px tall',
		bug: 'N6'
	}),
	// /ajustes/plantillas/nueva
	...entries('/ajustes/plantillas/nueva', 'busy', ['form data busy'], {
		width: 'compact',
		reason: 'the compact template editor shows nothing while the templates load',
		bug: 'N5'
	}),
	// /ajustes/recordatorios
	...entries('/ajustes/recordatorios', 'inert', ['new rule'], {
		reason: 'the page behind an open BottomSheet is not inert',
		bug: 'L16'
	}),
	// /ajustes/servicios
	...entries('/ajustes/servicios', 'inert', ['new service'], {
		reason: 'the page behind an open BottomSheet is not inert',
		bug: 'L16'
	}),
	// /citas/[id]
	...entries('/citas/[id]', 'busy', ['photos busy'], {
		reason: 'no loading line while the photos load',
		bug: 'N4'
	}),
	...entries('/citas/[id]', 'scroll', ['base', 'photos empty', 'photos error'], {
		width: 'compact',
		reason: 'a one-word 60-letter name pushes the page wider than the screen',
		bug: 'M5'
	}),
	...entries('/citas/[id]', 'text-overflow', ['base', 'photos empty', 'photos error'], {
		target: NAME,
		reason: 'a one-word 60-letter name spills out of its box',
		bug: 'M5'
	}),
	// /citas/nueva
	...entries('/citas/nueva', 'inert', ['quick-add client'], {
		reason: 'the page behind an open BottomSheet is not inert',
		bug: 'L16'
	}),
	// /clientas
	...entries('/clientas', 'scroll', ['base'], {
		width: 'compact',
		reason: 'a one-word 60-letter name pushes the page wider than the screen',
		bug: 'M5'
	}),
	...entries('/clientas', 'text-overflow', ['base'], {
		target: NAME,
		reason: 'a one-word 60-letter name spills out of its box',
		bug: 'M5'
	}),
	// /clientas/[id]
	...entries('/clientas/[id]', 'busy', ['reminder log busy'], {
		reason: 'no loading line while the log loads',
		bug: 'L11'
	}),
	...entries('/clientas/[id]', 'busy', ['photos busy'], {
		reason: 'no loading line while the photos load',
		bug: 'N4'
	}),
	...entries('/clientas/[id]', 'inert', ['archive'], {
		reason: 'the page behind an open BottomSheet is not inert',
		bug: 'L16'
	}),
	...entries(
		'/clientas/[id]',
		'scroll',
		['archive', 'base', 'photos empty', 'photos error', 'reminder log empty', 'reminder log error'],
		{ reason: 'a one-word 60-letter name pushes the page wider than the screen', bug: 'M5' }
	),
	...entries(
		'/clientas/[id]',
		'target-size',
		['archive', 'base', 'photos empty', 'photos error', 'reminder log empty', 'reminder log error'],
		{
			width: 'compact',
			target: 'Volver',
			reason: 'the back button shrinks to 22 px beside a long name',
			bug: 'M6'
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
			target: NAME,
			reason: 'a one-word 60-letter name spills out of its box',
			bug: 'M5'
		}
	),
	// /clientas/nueva
	...entries('/clientas/nueva', 'text-overflow', ['base'], {
		width: 'wide',
		target: NAME,
		reason: 'a one-word 60-letter name spills out of its box',
		bug: 'M5'
	}),
	// /enviar
	...entries('/enviar', 'busy', ['sent log busy'], {
		reason: 'no loading line while the log loads',
		bug: 'L11'
	}),
	...entries('/enviar', 'busy', ['due list busy'], {
		reason: 'no loading line while the due list loads',
		bug: 'N3'
	}),
	...entries('/enviar', 'clip', ['base', 'sent log empty', 'sent log error'], {
		width: 'wide',
		target: NAME,
		reason: 'the message preview cuts a one-word 60-letter name',
		bug: 'M5'
	}),
	...entries('/enviar', 'scroll', ['base', 'sent log empty', 'sent log error'], {
		width: 'compact',
		reason: 'a one-word 60-letter name pushes the page wider than the screen',
		bug: 'M5'
	}),
	...entries('/enviar', 'text-overflow', ['base', 'sent log empty', 'sent log error'], {
		width: 'compact',
		target: NAME,
		reason: 'a one-word 60-letter name spills out of its box',
		bug: 'M5'
	})
];
