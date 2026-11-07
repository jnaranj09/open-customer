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
			// The script, style and fetch sources (devsecops.md decision D8). The SPA's pages are
			// written at build time, so SvelteKit puts this policy in a <meta> tag with the hashes of
			// its inline scripts; the Go server adds the header-only directives (frame-ancestors) in
			// internal/httpapi/middleware.go. A browser enforces both.
			csp: {
				mode: 'hash',
				directives: {
					'default-src': ['self'],
					'script-src': ['self'],
					'style-src': ['self'],
					// Widening: inline style attributes. Svelte writes a component's static style="..."
					// into the HTML it clones, and app.html's wrapper div has one; neither can be hashed
					// (a hash for an attribute needs 'unsafe-hashes' and the exact text). Style
					// elements and every script stay 'self' or hashed.
					'style-src-attr': ['unsafe-inline'],
					// blob: for the photo previews made on the phone, data: for small inlined images.
					'img-src': ['self', 'blob:', 'data:'],
					'connect-src': ['self'],
					'worker-src': ['self'],
					'manifest-src': ['self']
				}
			},
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
