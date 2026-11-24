<script>
  import { onMount } from 'svelte';
  import { session, restoreSession } from './lib/session.svelte.js';
  import { loadSettings } from './lib/settings.svelte.js';
  import { initTheme } from './lib/theme.svelte.js';
  import { router } from './lib/router.svelte.js';
  import AppShell from './components/AppShell.svelte';
  import Login from './routes/Login.svelte';
  import Cashier from './routes/Cashier.svelte';
  import Inventory from './routes/Inventory.svelte';
  import Dashboard from './routes/Dashboard.svelte';
  import Users from './routes/Users.svelte';
  import Cashiers from './routes/Cashiers.svelte';
  import Settings from './routes/Settings.svelte';

  const routes = {
    '/': Cashier,
    '/stok': Inventory,
    '/ringkasan': Dashboard,
    '/akun-kasir': Cashiers,
    '/pengaturan': Settings,
    '/pengguna': Users,
  };
  // Who may open what. Hiding a page is only a convenience: the API enforces roles on its own.
  //   admin   the company: only the list of SOHO customers, never a shop's data
  //   owner   one shop's owner: everything of their own shop
  //   cashier a kasir seat: the till and nothing else
  const access = {
    '/': ['owner', 'cashier'],
    '/stok': ['owner'],
    '/ringkasan': ['owner'],
    '/akun-kasir': ['owner'],
    '/pengaturan': ['owner'],
    '/pengguna': ['admin'],
  };
  const home = { admin: '/pengguna', owner: '/', cashier: '/' };

  onMount(() => {
    initTheme();
    restoreSession();
  });

  // Receipts and the cart need the shop's profile and tax rules as soon as someone from a shop is signed in.
  $effect(() => {
    if (session.user && session.user.role !== 'admin') loadSettings();
  });

  const role = $derived(session.user?.role);
  const allowed = $derived(access[router.path]?.includes(role) ?? false);
  const Page = $derived(allowed ? routes[router.path] : routes[home[role] ?? '/']);
</script>

{#if !session.ready}
  <div class="grid min-h-dvh place-items-center text-muted">Memuat…</div>
{:else if !session.user}
  <Login />
{:else}
  <AppShell>
    <Page />
  </AppShell>
{/if}
