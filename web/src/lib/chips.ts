// The only template tokenizer on the phone: the ChipEditor and the attribute archive warning both
// use it. It matches the scanner in `internal/templates/parse.go` (design doc "Message templates"):
//
//	token    := "{{" sp* ns "." key sp* ( "|" sp* '"' fallback '"' sp* )? "}}"
//	ns       := [a-z]+          key := [a-z][a-z0-9_]*
//	fallback := any character except '"'     sp := ' ' (ASCII space only)
//
// The server stays the authority on save. Here a "{{" that starts no valid variable, and a variable
// of a namespace the editor never writes, is plain text: the editor shows it as typed and the
// server's answer on save says what is wrong with it.

type Ns = 'client' | 'appointment' | 'business';

export type Piece =
	{ kind: 'text'; text: string } | { kind: 'var'; ns: Ns; key: string; fallback: string | null };

const NAMESPACES: readonly string[] = ['client', 'appointment', 'business'] satisfies Ns[];

const isNs = (ns: string): ns is Ns => NAMESPACES.includes(ns);
const isLower = (c: string | undefined): c is string => c !== undefined && c >= 'a' && c <= 'z';
const isDigit = (c: string | undefined): boolean => c !== undefined && c >= '0' && c <= '9';

type Token = { ns: string; key: string; fallback: string | null; end: number };

function skipSpaces(cs: readonly string[], from: number): number {
	let i = from;
	while (cs[i] === ' ') {
		i++;
	}
	return i;
}

/** Matches one token whose "{{" is at `start`; `end` is the index after its "}}". */
function matchToken(cs: readonly string[], start: number): Token | null {
	let i = skipSpaces(cs, start + 2);
	const nsStart = i;
	while (isLower(cs[i])) {
		i++;
	}
	if (i === nsStart || cs[i] !== '.') {
		return null;
	}
	const ns = cs.slice(nsStart, i).join('');
	i++;
	const keyStart = i;
	if (!isLower(cs[i])) {
		return null;
	}
	while (isLower(cs[i]) || isDigit(cs[i]) || cs[i] === '_') {
		i++;
	}
	const key = cs.slice(keyStart, i).join('');
	i = skipSpaces(cs, i);
	let fallback: string | null = null;
	if (cs[i] === '|') {
		i = skipSpaces(cs, i + 1);
		if (cs[i] !== '"') {
			return null;
		}
		i++;
		const fbStart = i;
		while (i < cs.length && cs[i] !== '"') {
			i++;
		}
		if (i >= cs.length) {
			return null;
		}
		fallback = cs.slice(fbStart, i).join('');
		i = skipSpaces(cs, i + 1);
	}
	if (cs[i] !== '}' || cs[i + 1] !== '}') {
		return null;
	}
	return { ns, key, fallback, end: i + 2 };
}

/** Appends text, joining it to a text piece before it. */
function pushText(out: Piece[], text: string): void {
	if (text === '') {
		return;
	}
	const last = out.at(-1);
	if (last?.kind === 'text') {
		last.text += text;
	} else {
		out.push({ kind: 'text', text });
	}
}

/** The pieces of a template body: text and variables, in body order. Adjacent text is one piece. */
export function parseBody(body: string): Piece[] {
	const cs = Array.from(body); // by character, as the server counts positions
	const out: Piece[] = [];
	let lit = 0;
	for (let i = 0; i < cs.length;) {
		if (cs[i] !== '{' || cs[i + 1] !== '{') {
			i++;
			continue;
		}
		const token = matchToken(cs, i);
		if (token !== null && isNs(token.ns)) {
			pushText(out, cs.slice(lit, i).join(''));
			out.push({ kind: 'var', ns: token.ns, key: token.key, fallback: token.fallback });
			i = token.end;
			lit = i;
		} else {
			// Not a variable the editor writes: stays in the running text. A whole token of an
			// unknown namespace is skipped as the server's scanner does; a bad "{{" skips its two braces.
			i = token === null ? i + 2 : token.end;
		}
	}
	pushText(out, cs.slice(lit).join(''));
	return out;
}

/** The template syntax of a list of pieces. The fallback is written with straight quotes. */
export function serializeBody(pieces: readonly Piece[]): string {
	return pieces
		.map((p) => {
			if (p.kind === 'text') {
				return p.text;
			}
			const fallback = p.fallback === null ? '' : ` | "${p.fallback}"`;
			return `{{${p.ns}.${p.key}${fallback}}}`;
		})
		.join('');
}

/** True when the body holds the variable `ns.key`, with or without a fallback. */
export function templateUses(body: string, ns: 'client' | 'appointment', key: string): boolean {
	return parseBody(body).some((p) => p.kind === 'var' && p.ns === ns && p.key === key);
}

/** Why a "Si no tiene" text cannot be written, or null. The syntax has no escape for `"`. */
export function fallbackError(text: string): 'quote' | null {
	return text.includes('"') ? 'quote' : null;
}

/** What the editor's field holds, in the shape of the browser's nodes but without the DOM, so the
 *  mapping to pieces is a pure function. A browser may wrap lines in blocks (`div`, `p`). */
export type DomNode =
	| { kind: 'text'; text: string }
	| { kind: 'chip'; ns: string; key: string; fallback: string | null }
	| { kind: 'br' }
	| { kind: 'block'; children: readonly DomNode[] };

/** The pieces the editor's field holds. A line break is a newline; each block starts on a new line. */
export function piecesFromDom(nodes: readonly DomNode[]): Piece[] {
	const out: Piece[] = [];
	const endsInNewline = (): boolean => {
		const last = out.at(-1);
		return last?.kind === 'text' && last.text.endsWith('\n');
	};
	const walk = (list: readonly DomNode[]): void => {
		for (const node of list) {
			switch (node.kind) {
				case 'text':
					pushText(out, node.text);
					break;
				case 'br':
					pushText(out, '\n');
					break;
				case 'chip':
					if (isNs(node.ns)) {
						out.push({ kind: 'var', ns: node.ns, key: node.key, fallback: node.fallback });
					}
					break;
				case 'block':
					if (out.length > 0 && !endsInNewline()) {
						pushText(out, '\n');
					}
					walk(node.children);
					break;
			}
		}
	};
	walk(nodes);
	return out;
}

/** The label a chip shows: the server's current one, else the label of an attribute the chip row no
 *  longer offers (an archived one), else the key. */
export function chipLabel(
	ns: string,
	key: string,
	vars: readonly { ns: string; key: string; label: string }[],
	attributes: readonly { scope: string; key: string; label: string }[]
): string {
	return (
		vars.find((v) => v.ns === ns && v.key === key)?.label ??
		attributes.find((a) => a.scope === ns && a.key === key)?.label ??
		key
	);
}

/** The words a chip shows: its label, then its fallback when it has one ("Bebida favorita · algo rico"). */
export function chipText(label: string, fallback: string | null): string {
	return fallback === null || fallback === '' ? label : `${label} · ${fallback}`;
}

/** What is wrong with the editor's text before it is saved. The syntax has no escape for braces,
 *  so the saved body would not mean what the editor shows: `braces` is a "{{" in the text, or a
 *  "{" right before a variable (which reads as "{{"); `unknown` is a chip whose variable no longer
 *  exists. Each is named once, in the order first found. */
export type BodyProblem = 'braces' | 'unknown';

export function bodyProblems(
	pieces: readonly Piece[],
	known: (ns: string, key: string) => boolean
): BodyProblem[] {
	const found: BodyProblem[] = [];
	const add = (p: BodyProblem): void => {
		if (!found.includes(p)) {
			found.push(p);
		}
	};
	pieces.forEach((piece, i) => {
		if (piece.kind === 'var') {
			if (!known(piece.ns, piece.key)) {
				add('unknown');
			}
		} else if (
			piece.text.includes('{{') ||
			(piece.text.endsWith('{') && pieces[i + 1]?.kind === 'var')
		) {
			add('braces');
		}
	});
	return found;
}

/** True when the variable is one the chip row offers or an attribute (archived ones too), the
 *  same rule the server applies on save. */
export function chipKnown(
	ns: string,
	key: string,
	vars: readonly { ns: string; key: string }[],
	attributes: readonly { scope: string; key: string }[]
): boolean {
	return (
		vars.some((v) => v.ns === ns && v.key === key) ||
		attributes.some((a) => a.scope === ns && a.key === key)
	);
}
