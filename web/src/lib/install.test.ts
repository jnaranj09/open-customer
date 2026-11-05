import { describe, expect, it } from 'vitest';
import { needsInstallGuide } from './install';

const IPHONE_SAFARI =
	'Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1';
const DESKTOP_CHROME =
	'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36';

describe('needsInstallGuide', () => {
	it('is true on an iPhone in Safari, not installed', () => {
		expect(needsInstallGuide(IPHONE_SAFARI, false)).toBe(true);
	});

	it('is false once installed (standalone)', () => {
		expect(needsInstallGuide(IPHONE_SAFARI, true)).toBe(false);
	});

	it('is false on desktop Chrome', () => {
		expect(needsInstallGuide(DESKTOP_CHROME, false)).toBe(false);
	});
});
