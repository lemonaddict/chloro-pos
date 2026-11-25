// Build-time helpers for vite.config.js. Plain functions, so they can be tested with `node`.

/**
 * Checks VITE_API_BASE and returns it as an origin ('' when unset).
 * It must be just an origin: the client adds /api/v1 itself.
 */
export function normalizeApiBase(raw) {
  const value = (raw ?? '').trim().replace(/\/+$/, '');
  if (value === '') return '';
  let url;
  try {
    url = new URL(value);
  } catch {
    throw new Error(`VITE_API_BASE "${raw}" is not a URL. Use something like https://api.example.com`);
  }
  if ((url.protocol !== 'https:' && url.protocol !== 'http:') || url.pathname !== '/' || url.search || url.hash || url.username) {
    throw new Error(`VITE_API_BASE "${raw}" must be only scheme://host[:port], without a path such as /api/v1. Example: https://api.example.com`);
  }
  return url.origin;
}

/**
 * The CSP for a Static Site. The Go server sends an equivalent header when it serves the app itself
 * (internal/platform/middleware SecurityHeaders); a Static Site has no such header, so the build
 * puts it in a <meta> tag instead, with connect-src opened for the API and nothing else.
 */
export function contentSecurityPolicy(apiBase) {
  return [
    "default-src 'self'",
    "script-src 'self'",
    "style-src 'self'",
    "img-src 'self' data:",
    `connect-src 'self' ${apiBase}`,
    "object-src 'none'",
    "base-uri 'self'",
    "form-action 'self'",
  ].join('; ');
}
