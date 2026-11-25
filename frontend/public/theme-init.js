// Runs in <head> before the first paint (a plain, blocking script, so it also satisfies a CSP with
// script-src 'self'). It applies the saved theme so the page never flashes the wrong colours.
// Keep the rule in step with src/lib/theme.svelte.js: a saved 'light' or 'dark' wins, anything else
// follows the system. The storage key is the same one.
(function () {
  var dark = false;
  try {
    var saved = localStorage.getItem('chloropos-theme');
    dark = saved === 'dark' || (saved !== 'light' && matchMedia('(prefers-color-scheme: dark)').matches);
  } catch (e) {
    /* storage blocked: stay on the light theme until the app starts and reads the system setting */
  }
  var root = document.documentElement;
  root.classList.toggle('dark', dark);
  root.classList.toggle('light', !dark);
  root.style.colorScheme = dark ? 'dark' : 'light';
})();
