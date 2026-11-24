// One small fetch wrapper: JSON in, JSON out, CSRF header on every change, friendly errors.
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

async function send(method, path, body) {
  const headers = { Accept: 'application/json' };
  if (body !== undefined) headers['Content-Type'] = 'application/json';
  if (method !== 'GET') headers['X-CSRF-Token'] = csrfToken;
  try {
    return await fetch(`/api/v1${path}`, {
      method,
      headers,
      credentials: 'same-origin',
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
  return res.status === 204 ? null : await res.json().catch(() => null);
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
