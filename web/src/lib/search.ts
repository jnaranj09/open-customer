export type Searchable = { id: string; name: string; phone_digits: string | null };

const MAX_RESULTS = 20;
const MIN_PHONE_DIGITS = 3;

/** Lowercase, accents removed (NFD, combining marks dropped): "Ána" becomes "ana". */
export function normalize(s: string): string {
	return s.normalize('NFD').replace(/\p{M}/gu, '').toLowerCase();
}

type Hit<T> = { item: T; name: string; key: string; startsWord: boolean };

const byKey = <T>(a: Hit<T>, b: Hit<T>): number => {
	if (a.key !== b.key) {
		return a.key < b.key ? -1 : 1;
	}
	if (a.name === b.name) {
		return 0;
	}
	return a.name < b.name ? -1 : 1;
};

/** The only client search. A client matches when the name contains the query (ignoring case and
 *  accents) or, for a query with 3 or more digits, when `phone_digits` contains them. Names where
 *  the query starts a word come first; each group is ordered by name. At most 20 results. A blank
 *  query matches nothing. The query is plain text: no character is a wildcard. */
export function search<T extends Searchable>(items: readonly T[], query: string): T[] {
	const q = normalize(query).trim();
	if (q === '') {
		return [];
	}
	const digits = query.replace(/\D/g, '');
	const byPhone = digits.length >= MIN_PHONE_DIGITS;
	const hits: Hit<T>[] = [];
	for (const item of items) {
		const key = normalize(item.name);
		const byName = key.includes(q);
		if (byName || (byPhone && item.phone_digits?.includes(digits) === true)) {
			hits.push({
				item,
				name: item.name,
				key,
				startsWord: byName && (key.startsWith(q) || key.includes(` ${q}`))
			});
		}
	}
	const first = hits.filter((h) => h.startsWord).sort(byKey);
	const rest = hits.filter((h) => !h.startsWord).sort(byKey);
	return [...first, ...rest].slice(0, MAX_RESULTS).map((h) => h.item);
}
