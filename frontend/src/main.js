import './app.css';
import { mount } from 'svelte';
import App from './App.svelte';
import { initTheme } from './lib/theme.svelte.js';

// Apply the theme and start following the system setting before anything renders.
// (/theme-init.js in <head> has already covered the very first paint.)
initTheme();

// index.html ships a "Memuat…" placeholder so the page is never blank while this bundle loads.
// App.svelte draws the same markup until the session is known, so swapping them shifts nothing.
const target = document.getElementById('app');
target.replaceChildren();

export default mount(App, { target });
