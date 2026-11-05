// Steps and helpers more than one e2e spec uses. jscpd scans tests/ with threshold 0, so a step two
// specs need lives here, not in both.

/** An es.json text with its `{name}` placeholders filled; a missing value throws. */
export function fill(text: string, params: Record<string, string>): string {
	return text.replace(/\{(\w+)\}/g, (_match, name: string) => {
		const value = params[name];
		if (value === undefined) {
			throw new Error(`fill: no value for {${name}}`);
		}
		return value;
	});
}
