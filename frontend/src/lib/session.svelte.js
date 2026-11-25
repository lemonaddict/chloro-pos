import { api, ApiError, CROSS_ORIGIN, setCsrfToken, setUnauthorizedHandler } from './api.js';
import { resetSettings } from './settings.svelte.js';

// The session cookie is httpOnly, so the page can't read it. It only learns who is signed in
// (and the CSRF token) from the API.
export const session = $state({ user: null, ready: false });

const COOKIE_BLOCKED =
  'Browser menolak cookie sesi dari server API karena aplikasi dan API ada di situs yang berbeda. ' +
  'Coba browser lain, atau minta admin memakai domain yang sama untuk aplikasi dan API.';

function apply(payload) {
  resetSettings();
  session.user = payload?.user ?? null;
  setCsrfToken(payload?.csrf_token);
}

export async function restoreSession() {
  try {
    apply(await api.get('/auth/me'));
  } catch {
    apply(null);
  } finally {
    session.ready = true;
  }
}

// When the API is on another site, some browsers (Safari, Chrome Incognito, Firefox strict mode)
// quietly drop its cookie: sign-in succeeds, then every request is 401 and the person is bounced
// back to the form with no explanation. Confirm the cookie came back before showing the app.
async function begin(payload) {
  if (CROSS_ORIGIN) {
    try {
      await api.get('/auth/me');
    } catch (e) {
      throw e instanceof ApiError && e.status === 401 ? new ApiError(401, 'cookie_blocked', COOKIE_BLOCKED) : e;
    }
  }
  apply(payload);
}

export async function signIn(username, password) {
  await begin(await api.post('/auth/login', { username, password }));
}

// A kasir signs in with the shop's username and the code the owner gave that kasir, never the owner's password.
export async function signInCashier(username, code) {
  await begin(await api.post('/auth/login/cashier', { username, code }));
}

export async function signOut() {
  try {
    await api.post('/auth/logout');
  } finally {
    apply(null);
  }
}

setUnauthorizedHandler(() => apply(null));
