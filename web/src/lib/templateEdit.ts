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
