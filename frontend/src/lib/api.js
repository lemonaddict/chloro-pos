// One small fetch wrapper: JSON in, JSON out, CSRF header on every change, friendly errors.

// Where the API lives. Empty means the same origin as the page: the Vite proxy in development, or
// the Go server serving the built app. Set VITE_API_BASE (build time, no trailing slash, for
// example https://api.example.com) when the app is a Static Site and the API is another service.
export const API_BASE = (import.meta.env.VITE_API_BASE ?? '').trim().replace(/\/+$/, '');

function isCrossOrigin(base) {
  if (!base) return false;
  try {
    return new URL(base, location.origin).origin !== location.origin;
  } catch {
    return true;
  }
}

// Another origin only receives the session cookie if the request opts in with 'include'.
export const CROSS_ORIGIN = isCrossOrigin(API_BASE);
const CREDENTIALS = CROSS_ORIGIN ? 'include' : 'same-origin';

let csrfToken = '';
let onUnauthorized = () => {};

export const setCsrfToken = (token) => { csrfToken = token || ''; };
export const setUnauthorizedHandler = (fn) => { onUnauthorized = fn; };

export class ApiError extends Error {
  constructor(status, code, message) {
    super(message);
    this.status = status;
    this.code = code;
  }
}

const NETWORK = 'Tidak bisa menghubungi server. Periksa koneksi lalu coba lagi.';
const GENERIC = 'Terjadi kesalahan. Coba lagi.';
const NOT_THE_API = 'Alamat server tidak menjawab seperti API ChloroPOS. Hubungi admin untuk memeriksa pengaturan alamat API.';

async function send(method, path, body) {
  const headers = { Accept: 'application/json' };
  if (body !== undefined) headers['Content-Type'] = 'application/json';
  if (method !== 'GET') headers['X-CSRF-Token'] = csrfToken;
  try {
    return await fetch(`${API_BASE}/api/v1${path}`, {
      method,
      headers,
      credentials: CREDENTIALS,
      body: body === undefined ? undefined : JSON.stringify(body),
    });
  } catch {
    throw new ApiError(0, 'network', NETWORK);
  }
}

async function fail(res, path) {
  const data = await res.json().catch(() => null);
  if (res.status === 401 && !path.startsWith('/auth/login')) onUnauthorized();
  return new ApiError(res.status, data?.error?.code ?? 'error', data?.error?.message ?? GENERIC);
}

async function request(method, path, body) {
  const res = await send(method, path, body);
  if (!res.ok) throw await fail(res, path);
  if (res.status === 204) return null;
  // A static host answers an unknown path with index.html (status 200). That means the API base is
  // wrong or missing; say so instead of treating the page as an empty reply.
  if (!(res.headers.get('content-type') ?? '').includes('application/json')) {
    throw new ApiError(res.status, 'not_the_api', NOT_THE_API);
  }
  return await res.json().catch(() => null);
}

export const api = {
  get: (path) => request('GET', path),
  post: (path, body) => request('POST', path, body ?? {}),
  put: (path, body) => request('PUT', path, body),
};

/** Downloads a file the API generates (for example the Excel report). */
export async function download(path, filename) {
  const res = await send('GET', path);
  if (!res.ok) throw await fail(res, path);
  const url = URL.createObjectURL(await res.blob());
  const link = document.createElement('a');
  link.href = url;
  link.download = filename;
  document.body.append(link);
  link.click();
  link.remove();
  setTimeout(() => URL.revokeObjectURL(url), 10_000);
}
