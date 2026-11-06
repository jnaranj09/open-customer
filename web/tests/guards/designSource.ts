// The design source behind the UI guards design-tokens and design-parity (.claude/rules/ui.md "UI
// pipeline"). It reads the Pencil file (JSON) and the name-map file, and fails closed: a shape it
// does not know throws, so a format change turns the guards red instead of letting them pass on
// nothing. Kept apart from sourceScan.ts, which reads code, not the design.
//
// The `.pen` shape read here: a top-level `variables` map (name → { type, value }) and a `children`
// tree whose reusable components are nodes with `"reusable": true` and a `name` like
// `Button/Primary` (component/variant).
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { WEB } from './sourceScan';

/** The repository root. */
const ROOT = join(WEB, '..');
/** The design source. */
export const PEN_FILE = join(ROOT, 'design/soft-glass.pen');
/** The name map. It holds exceptions only: a token whose CSS name is not `--<name>`, a component
 *  that does not follow `Component/Variant` → Component + variant. */
export const MAP_FILE = join(ROOT, 'design/design-map.json');

export type PenValue = string | number | boolean;

export type PenDesign = {
	/** Variable name → its type and value. */
	variables: Map<string, { type: string; value: PenValue }>;
	/** Names of every reusable component, sorted (duplicates kept, so a guard can flag them). */
	components: string[];
};

export type DesignMap = {
	/** `.pen` variable → CSS custom property, only where the names differ. */
	tokens: Record<string, string>;
	/** `.pen` component → primitive (path under the primitives folder, no extension) and its
	 *  variant, if any. Only where the default mapping (`X/Y` → X, variant y) is wrong. */
	components: Record<string, { component: string; variant?: string }>;
};

const isRecord = (v: unknown): v is Record<string, unknown> =>
	typeof v === 'object' && v !== null && !Array.isArray(v);

/** The variables and reusable components of a parsed `.pen` file; throws on any other shape. */
export function parsePen(json: unknown): PenDesign {
	if (!isRecord(json)) throw new Error('pen: not an object');
	const vars = json['variables'];
	if (!isRecord(vars)) throw new Error('pen: no variables map');
	const variables = new Map<string, { type: string; value: PenValue }>();
	for (const [name, v] of Object.entries(vars)) {
		if (!isRecord(v) || typeof v['type'] !== 'string') throw new Error(`pen: bad variable ${name}`);
		const value = v['value'];
		// A themed or computed value is a shape this guard does not read yet: extend it, do not skip.
		if (typeof value !== 'string' && typeof value !== 'number' && typeof value !== 'boolean') {
			throw new Error(`pen: variable ${name} has a value shape this guard does not read`);
		}
		variables.set(name, { type: v['type'], value });
	}
	if (!Array.isArray(json['children'])) throw new Error('pen: no children list');
	const components: string[] = [];
	const visit = (node: unknown): void => {
		if (Array.isArray(node)) {
			node.forEach(visit);
			return;
		}
		if (!isRecord(node)) return;
		if (node['reusable'] === true) {
			const name = node['name'];
			if (typeof name !== 'string' || name === '') {
				throw new Error('pen: reusable node with no name');
			}
			components.push(name);
		}
		Object.values(node).forEach(visit);
	};
	visit(json['children']);
	return { variables, components: components.sort() };
}

/** The name map; throws on any other shape. */
export function parseDesignMap(json: unknown): DesignMap {
	if (!isRecord(json) || !isRecord(json['tokens']) || !isRecord(json['components'])) {
		throw new Error('design map: needs a tokens and a components object');
	}
	const tokens: Record<string, string> = {};
	for (const [name, css] of Object.entries(json['tokens'])) {
		if (typeof css !== 'string' || !css.startsWith('--')) {
			throw new Error(`design map: token ${name} must map to a --custom-property`);
		}
		tokens[name] = css;
	}
	const components: DesignMap['components'] = {};
	for (const [name, entry] of Object.entries(json['components'])) {
		const component = isRecord(entry) ? entry['component'] : undefined;
		const variant = isRecord(entry) ? entry['variant'] : undefined;
		if (typeof component !== 'string' || component === '') {
			throw new Error(`design map: component ${name} needs a component name`);
		}
		if (variant !== undefined && typeof variant !== 'string') {
			throw new Error(`design map: component ${name} has a non-string variant`);
		}
		components[name] = variant === undefined ? { component } : { component, variant };
	}
	return { tokens, components };
}

export const readJson = (file: string): unknown => JSON.parse(readFileSync(file, 'utf8'));
