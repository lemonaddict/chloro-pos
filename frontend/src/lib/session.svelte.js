import { api, setCsrfToken, setUnauthorizedHandler } from './api.js';

// The session cookie is httpOnly, so the page can't read it. It only learns who is signed in
// (and the CSRF token) from the API.
export const session = $state({ user: null, ready: false });

function apply(payload) {
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

export async function signIn(username, password) {
  apply(await api.post('/auth/login', { username, password }));
}

// A kasir signs in with the shop's username and the code the owner gave that kasir, never the owner's password.
export async function signInCashier(username, code) {
  apply(await api.post('/auth/login/cashier', { username, code }));
}

export async function signOut() {
  try {
    await api.post('/auth/logout');
  } finally {
    apply(null);
  }
}

setUnauthorizedHandler(() => apply(null));
