// Theme: 'system' | 'light' | 'dark'. Stored in localStorage so the choice survives reload.
const STORAGE_KEY = 'chloropos-theme';

function readStored() {
  try {
    const v = localStorage.getItem(STORAGE_KEY);
    if (v === 'light' || v === 'dark' || v === 'system') return v;
  } catch {
    /* private mode / SSR */
  }
  return 'system';
}

/** @type {{ preference: 'system' | 'light' | 'dark' }} */
export const theme = $state({
  preference: /** @type {'system' | 'light' | 'dark'} */ (readStored()),
});

export function resolvedTheme() {
  if (theme.preference === 'system') {
    return typeof window !== 'undefined' && window.matchMedia('(prefers-color-scheme: dark)').matches
      ? 'dark'
      : 'light';
  }
  return theme.preference;
}

function applyDom(resolved) {
  const root = document.documentElement;
  root.classList.toggle('dark', resolved === 'dark');
  root.classList.toggle('light', resolved === 'light');
  root.style.colorScheme = resolved;
}

/** Apply current preference to <html> and keep it in sync with system when preference is 'system'. */
export function initTheme() {
  applyDom(resolvedTheme());

  const mq = window.matchMedia('(prefers-color-scheme: dark)');
  const onChange = () => {
    if (theme.preference === 'system') applyDom(resolvedTheme());
  };
  mq.addEventListener('change', onChange);
  return () => mq.removeEventListener('change', onChange);
}

/** Set theme explicitly: 'system' | 'light' | 'dark'. */
export function setTheme(next) {
  if (next !== 'system' && next !== 'light' && next !== 'dark') return;
  theme.preference = next;
  try {
    localStorage.setItem(STORAGE_KEY, next);
  } catch {
    /* ignore */
  }
  applyDom(resolvedTheme());
}

/** Cycle system → light → dark → system. */
export function cycleTheme() {
  const order = /** @type {const} */ (['system', 'light', 'dark']);
  const i = order.indexOf(theme.preference);
  setTheme(order[(i + 1) % order.length]);
}
