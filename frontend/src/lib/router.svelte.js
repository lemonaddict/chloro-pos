// A tiny history-based router. The Go server returns index.html for unknown paths, so refresh works.
export const router = $state({ path: window.location.pathname });

export function navigate(to) {
  if (to === router.path) return;
  window.history.pushState({}, '', to);
  router.path = to;
}

window.addEventListener('popstate', () => {
  router.path = window.location.pathname;
});
