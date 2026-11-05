/** The focus trap's wrap rule: the index to focus after Tab (or Shift+Tab) among `count`
 *  focusable elements, `current` being the focused one's index or -1 when focus is outside the
 *  trap. Returns -1 when there is nothing to focus. */
export function nextFocus(current: number, count: number, shift: boolean): number {
	if (count <= 0) {
		return -1;
	}
	if (shift) {
		return current <= 0 ? count - 1 : current - 1;
	}
	return current + 1 >= count ? 0 : current + 1;
}
