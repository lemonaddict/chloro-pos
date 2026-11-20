<script>
  import { onMount } from 'svelte';
  import { session, restoreSession } from './lib/session.svelte.js';
  import { router } from './lib/router.svelte.js';
  import AppShell from './components/AppShell.svelte';
  import Login from './routes/Login.svelte';
  import Cashier from './routes/Cashier.svelte';
  import Inventory from './routes/Inventory.svelte';
  import Dashboard from './routes/Dashboard.svelte';

  const routes = { '/': Cashier, '/inventory': Inventory, '/dashboard': Dashboard };
  const ownerOnly = new Set(['/inventory', '/dashboard']);

  onMount(restoreSession);

  // Hiding a page is only a convenience. The API enforces roles on its own.
  const allowed = $derived(!ownerOnly.has(router.path) || session.user?.role === 'owner');
  const Page = $derived(allowed ? (routes[router.path] ?? Cashier) : Cashier);
</script>

{#if !session.ready}
  <div class="grid min-h-dvh place-items-center text-muted">Loading…</div>
{:else if !session.user}
  <Login />
{:else}
  <AppShell>
    <Page />
  </AppShell>
{/if}
