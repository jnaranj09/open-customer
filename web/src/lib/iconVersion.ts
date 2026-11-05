import { writable } from 'svelte/store';

// Bumped after an icon upload. BusinessIcon adds it to the icon URL, so an image already on screen
// reloads: the URL never changes otherwise, and the browser would keep showing the old picture.
export const iconVersion = writable(0);
