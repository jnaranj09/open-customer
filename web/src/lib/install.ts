const IOS = /iPhone|iPad|iPod/;

/** True on iOS outside the installed app: the one place she must add the app to the Home Screen
 *  by hand before push works. `standalone` is `navigator.standalone` or the display-mode query.
 *  An iPad that reports a desktop Mac user agent is not detected. */
export function needsInstallGuide(ua: string, standalone: boolean): boolean {
	return IOS.test(ua) && !standalone;
}
