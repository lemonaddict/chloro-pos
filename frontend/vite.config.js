import { defineConfig, loadEnv } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import tailwindcss from '@tailwindcss/vite';
import { normalizeApiBase, contentSecurityPolicy } from './build-helpers.js';

// Adds the CSP <meta> tag to the built index.html. Only used when the API is on another origin.
const cspMeta = (apiBase) => ({
  name: 'chloropos-csp-meta',
  apply: 'build',
  transformIndexHtml: () => [
    {
      tag: 'meta',
      attrs: { 'http-equiv': 'Content-Security-Policy', content: contentSecurityPolicy(apiBase) },
      injectTo: 'head-prepend',
    },
  ],
});

export default defineConfig(({ mode }) => {
  // VITE_API_BASE is read at build time and baked into the bundle. Empty (the default) means the
  // API is on the same origin: the dev proxy below, or the Go server serving the built app.
  const apiBase = normalizeApiBase(loadEnv(mode, process.cwd(), 'VITE_').VITE_API_BASE);

  return {
    plugins: [tailwindcss(), svelte(), ...(apiBase ? [cspMeta(apiBase)] : [])],
    server: {
      port: 5173,
      // In development the browser talks to Vite, which forwards /api to the Go server, so cookies
      // and CSRF behave exactly as in production.
      // changeOrigin must stay false: the string shorthand ('/api': 'http://...') switches it on,
      // which rewrites Host to localhost:8080 while Origin stays localhost:5173, and the Go origin
      // check then (rightly) rejects every POST.
      proxy: {
        '/api': { target: 'http://localhost:8080', changeOrigin: false },
      },
    },
    build: { target: 'es2022', sourcemap: false },
  };
});
