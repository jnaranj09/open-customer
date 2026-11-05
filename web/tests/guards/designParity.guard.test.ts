// design-parity (.claude/rules/ui.md "UI pipeline", stage 3): every reusable component in the
// `.pen` file maps to one primitive in components/ui, and every primitive maps back to at least one
// design component. Default mapping: `Button/Primary` → primitive `Button`, variant `primary`
// (first segment = primitive, the rest = variant with its first letter lowercased); `Card` →
// `Card`. The name map (`components` in design/design-map.json) holds only the exceptions; an entry
// that repeats the default, or names a design component that is gone, fails. Two allow-lists, each
// only shrinks and fails on a stale entry: CODE_ONLY (a primitive with no design: behaviour that
// renders nothing of its own) and DESIGN_ONLY (a design component not built yet). The variant name
// is checked by each primitive's own prop type at its call sites; this guard checks names only.
import { mkdirSync, mkdtempSync, readdirSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, relative, sep } from 'node:path';
import { describe, expect, it } from 'vitest';
import { MAP_FILE, PEN_FILE, parseDesignMap, parsePen, readJson } from './designSource';
import type { DesignMap } from './designSource';
import { SRC } from './sourceScan';

/** The primitives folder. Only files here may carry surface and radius classes (eslint). */
const PRIMITIVES = join(SRC, 'lib/components/ui');
const COMPONENT_EXT = '.svelte';

/** Primitives with no design component. Each: why. Only shrinks. */
const CODE_ONLY: Record<string, string> = {};

/** Design components with no primitive yet. Each: why. Only shrinks. */
const DESIGN_ONLY: Record<string, string> = {
	AppointmentRow: 'not built: the agenda draws appointments in TimeGrid, no list of rows'
};

/** Component names under the primitives folder: path relative to it, no extension, `/`-separated.
 *  Tests and stories are not primitives. */
export function primitiveNames(dir: string): string[] {
	const out: string[] = [];
	const walk = (d: string): void => {
		for (const e of readdirSync(d, { withFileTypes: true })) {
			const p = join(d, e.name);
			if (e.isDirectory()) walk(p);
			else if (
				e.name.endsWith(COMPONENT_EXT) &&
				!e.name.includes('.test.') &&
				!e.name.includes('.stories.')
			) {
				out.push(relative(dir, p).slice(0, -COMPONENT_EXT.length).split(sep).join('/'));
			}
		}
	};
	walk(dir);
	return out.sort();
}

/** The primitive a design component maps to when the name map says nothing. */
export function defaultMapping(name: string): { component: string; variant?: string } {
	const [component = name, ...rest] = name.split('/');
	const variant = rest.join('/');
	if (variant === '') return { component };
	return { component, variant: variant.charAt(0).toLowerCase() + variant.slice(1) };
}

type Drift = {
	duplicate: string[];
	staleMap: string[];
	redundantMap: string[];
	missingFile: string[];
	orphan: string[];
	staleCodeOnly: string[];
	staleDesignOnly: string[];
};

export function parityDrift(
	penComponents: readonly string[],
	map: DesignMap['components'],
	primitives: readonly string[],
	codeOnly: Record<string, string>,
	designOnly: Record<string, string>
): Drift {
	const inPen = new Set(penComponents);
	const resolved = [...inPen].map((n) => [n, map[n] ?? defaultMapping(n)] as const);
	const files = new Set(primitives);
	const built = resolved.filter(([n]) => designOnly[n] === undefined);
	const mapped = new Set(built.map(([, m]) => m.component));
	return {
		duplicate: penComponents.filter((n, i) => penComponents.indexOf(n) !== i),
		staleMap: Object.keys(map).filter((n) => !inPen.has(n)),
		redundantMap: Object.entries(map)
			.filter(([n, m]) => {
				const d = defaultMapping(n);
				return m.component === d.component && m.variant === d.variant;
			})
			.map(([n]) => n),
		missingFile: built
			.filter(([, m]) => !files.has(m.component))
			.map(([n, m]) => `${n} → ${m.component}`),
		orphan: primitives.filter((f) => !mapped.has(f) && codeOnly[f] === undefined),
		staleCodeOnly: Object.keys(codeOnly).filter((f) => mapped.has(f) || !files.has(f)),
		staleDesignOnly: resolved
			.filter(([n, m]) => designOnly[n] !== undefined && files.has(m.component))
			.map(([n]) => n)
			.concat(Object.keys(designOnly).filter((n) => !inPen.has(n)))
	};
}

const clean: Drift = {
	duplicate: [],
	staleMap: [],
	redundantMap: [],
	missingFile: [],
	orphan: [],
	staleCodeOnly: [],
	staleDesignOnly: []
};

describe('defaultMapping', () => {
	it('splits Component/Variant and lowercases the first letter of the variant', () => {
		expect(defaultMapping('Button/Primary')).toEqual({ component: 'Button', variant: 'primary' });
		expect(defaultMapping('DueCard/BookingPrompt')).toEqual({
			component: 'DueCard',
			variant: 'bookingPrompt'
		});
		expect(defaultMapping('Card')).toEqual({ component: 'Card' });
	});
});

describe('primitiveNames', () => {
	it('keys by path under the folder and skips tests and stories', () => {
		const dir = mkdtempSync(join(tmpdir(), 'parity-'));
		try {
			mkdirSync(join(dir, 'forms'));
			for (const f of ['Button', 'Button.test', 'Button.stories', 'forms/Input']) {
				writeFileSync(join(dir, `${f}${COMPONENT_EXT}`), '');
			}
			expect(primitiveNames(dir)).toEqual(['Button', 'forms/Input']);
		} finally {
			rmSync(dir, { recursive: true, force: true });
		}
	});
});

describe('parityDrift', () => {
	it('is clean when both sides match by default names, with one exception mapped', () => {
		expect(
			parityDrift(
				['Button/Ghost', 'Button/Primary', 'Card', 'Row', 'TextField'],
				{ TextField: { component: 'forms/Input' } },
				['Button', 'Card', 'forms/Input'],
				{},
				{ Row: 'probe' }
			)
		).toEqual(clean);
	});

	it('flags each kind of drift', () => {
		expect(
			parityDrift(
				['Button/Primary', 'Button/Primary', 'Chip', 'Card', 'Built', 'Unbuilt'],
				{ Gone: { component: 'Old' }, Card: { component: 'Card' } },
				['Built', 'Button', 'Card', 'Stray', 'Trap'],
				{ Trap: 'probe', Card: 'probe' },
				{ Built: 'probe', Missing: 'probe', Unbuilt: 'probe' }
			)
		).toEqual({
			duplicate: ['Button/Primary'],
			staleMap: ['Gone'],
			redundantMap: ['Card'],
			missingFile: ['Chip → Chip'],
			orphan: ['Built', 'Stray'],
			staleCodeOnly: ['Card'],
			staleDesignOnly: ['Built', 'Missing']
		});
	});
});

describe('design-parity guard', () => {
	// Read inside the tests, not at module load: a parse failure is one red test with its message.
	const load = (): { pen: string[]; map: DesignMap['components']; primitives: string[] } => ({
		pen: parsePen(readJson(PEN_FILE)).components,
		map: parseDesignMap(readJson(MAP_FILE)).components,
		primitives: primitiveNames(PRIMITIVES)
	});

	it('finds design components and primitives (a walk that finds none is broken)', () => {
		const { pen, primitives } = load();
		expect(pen.length).toBeGreaterThan(0);
		expect(primitives.length).toBeGreaterThan(0);
	});

	it('every design component has one primitive and every primitive has a design', () => {
		// A new component starts in the .pen file, then gets its primitive (and a map entry only if
		// its name breaks the default) in the same change. Never allow-list a primitive to get green.
		const { pen, map, primitives } = load();
		expect(parityDrift(pen, map, primitives, CODE_ONLY, DESIGN_ONLY)).toEqual(clean);
	});
});
