import adapter from '@sveltejs/adapter-static';
import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vitest/config';

// The Go server owns the API, the manifest and the icons; `make dev` runs it on :8080.
const goServer = 'http://localhost:8080';

export default defineConfig({
	plugins: [
		tailwindcss(),
		sveltekit({
			// SPA: every unknown path gets index.html; the Go server serves it (design doc item 35).
			adapter: adapter({ fallback: 'index.html' }),
			serviceWorker: { register: true },
			// The same value as the Go -ldflags version; the Makefile and the Dockerfile pass VERSION.
			version: { name: process.env['VERSION'] ?? 'dev' }
		})
	],
	server: {
		port: 5173,
		strictPort: true,
		proxy: {
			'/api': goServer,
			'/manifest.webmanifest': goServer,
			'/icons': goServer,
			'/apple-touch-icon.png': goServer
		}
	},
	test: {
		include: ['src/**/*.test.ts', 'tests/guards/**/*.test.ts'],
		environment: 'node',
		allowOnly: false,
		restoreMocks: true
	}
});
