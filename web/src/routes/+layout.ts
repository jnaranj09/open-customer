// The app is a client-only PWA: the server serves one shell and the service worker answers every
// navigation from it (design doc items 64-65, 71).
export const ssr = false;
export const prerender = false;
