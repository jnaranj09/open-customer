/** The catalog rows (services, attributes) in the order the server keeps them: by `position`, then
 *  by name. The cache reads rows by id, so the phone sorts them once when it reads them. */
export function byPosition<T extends { position: number }>(
	rows: readonly T[],
	name: (row: T) => string
): T[] {
	return [...rows].sort((a, b) => a.position - b.position || name(a).localeCompare(name(b), 'es'));
}
