// Client hooks: SvelteKit runs init once, before the app starts.
import { z } from 'zod';

export function init(): void {
	// zod compiles its object parsers with new Function when it is allowed to. The CSP has no
	// 'unsafe-eval' (vite.config.ts), so its probe for that is a reported CSP violation on every
	// page; jitless skips the probe and the compiled parsers.
	z.config({ jitless: true });
}
