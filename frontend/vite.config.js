import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import tailwindcss from '@tailwindcss/vite';

export default defineConfig({
  plugins: [tailwindcss(), svelte()],
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
});
