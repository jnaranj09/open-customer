/** True when the editor holds changes that are not saved. A new template (no `saved`) is clean
 *  while its name and body are empty. The name is compared as save sends it (trimmed); the body
 *  as typed, because it is stored exactly as sent. */
export function templateDirty(
	saved: { name: string; body: string } | undefined,
	name: string,
	body: string
): boolean {
	if (saved === undefined) {
		return name.trim() !== '' || body !== '';
	}
	return name.trim() !== saved.name || body !== saved.body;
}

/** Why the form lets a navigation pass without asking: she chose to discard, or the form is about
 *  to navigate itself after a save (create) or an archive, with what it sent. */
export type Exemption = { kind: 'discard' } | { kind: 'saved'; name: string; body: string };

/** True when a navigation may go without the "¿Descartar los cambios?" question. An update or a
 *  restore stays on the page and sets no exemption, so the guard stays on. A save that navigates
 *  passes only while nothing changed since what was sent (typing during the request still asks). */
export function navigationAllowed(
	exemption: Exemption | null,
	dirty: boolean,
	name: string,
	body: string
): boolean {
	if (!dirty || exemption?.kind === 'discard') {
		return true;
	}
	return exemption?.kind === 'saved' && name.trim() === exemption.name && body === exemption.body;
}
