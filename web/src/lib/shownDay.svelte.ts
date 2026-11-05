// The day Hoy is showing, for the sidebar's "Nueva cita" (design doc item 70): the link opens the
// form on that day. Hoy sets it while it is on screen and clears it on leaving, so every other
// page falls back to today.
let current = $state<string | null>(null);

export const shownDay = {
	/** The day on Hoy now, or null when Hoy is not open. */
	get current(): string | null {
		return current;
	},
	set(day: string | null): void {
		current = day;
	}
};
