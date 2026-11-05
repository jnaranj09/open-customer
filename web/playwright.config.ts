import { mkdtempSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { defineConfig, devices } from '@playwright/test';
import { AUTH_FILE, BASE_URL, ZONE, appEnv } from './tests/e2e/env';

// End-to-end tests (design doc item 79): WebKit on an iPhone profile against the release binary
// and a fresh database. Every spec shares that one database and the per-spec reset truncates
// tables, so the run is serial. Never raise retries or timeouts to get green.

// The photos of this run. Every Playwright process loads this file, so each one removes the folder
// it made when it exits.
const mediaDir = mkdtempSync(join(tmpdir(), 'oc-e2e-media-'));
process.on('exit', () => {
	rmSync(mediaDir, { recursive: true, force: true });
});

export default defineConfig({
	testDir: 'tests/e2e',
	workers: 1,
	fullyParallel: false,
	forbidOnly: true,
	retries: 0,
	reporter: 'list',
	use: {
		...devices['iPhone 15'],
		baseURL: BASE_URL,
		timezoneId: ZONE,
		locale: 'es-CR'
	},
	projects: [
		// CUJ 1 and 2 in the real wizard and screens; saves the logged-in state for the rest.
		{ name: 'setup', testMatch: /setup\.spec\.ts/ },
		{
			name: 'cujs',
			testIgnore: /setup\.spec\.ts/,
			dependencies: ['setup'],
			use: { storageState: AUTH_FILE }
		}
	],
	// The binary applies the migrations itself on start; the database is fresh, so the migration
	// guard has no backup to check.
	webServer: {
		command: '../bin/open-customer serve',
		url: `${BASE_URL}/readyz`,
		env: appEnv(mediaDir),
		reuseExistingServer: false
	}
});
