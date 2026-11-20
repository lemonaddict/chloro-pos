<script>
  import { session, signOut } from '../lib/session.svelte.js';
  import { router, navigate } from '../lib/router.svelte.js';

  let { children } = $props();

  const items = [
    { path: '/', label: 'Sell', owner: false, icon: 'M6 7h12l-1 12H7L6 7Zm3 0a3 3 0 0 1 6 0' },
    { path: '/inventory', label: 'Stock', owner: true, icon: 'M4 8l8-4 8 4v8l-8 4-8-4V8Zm0 0 8 4 8-4M12 12v8' },
    { path: '/dashboard', label: 'Reports', owner: true, icon: 'M5 20V10m7 10V4m7 16v-7' },
  ];
  const visible = $derived(items.filter((i) => !i.owner || session.user?.role === 'owner'));

  function go(event, path) {
    event.preventDefault();
    navigate(path);
  }
</script>

<div class="flex h-dvh flex-col md:flex-row print:block print:h-auto">
  <nav
    aria-label="Main"
    class="order-last flex shrink-0 items-center justify-around gap-1 border-t border-line bg-panel p-2 md:order-first md:w-28 md:flex-col md:justify-start md:gap-2 md:border-r md:border-t-0 md:px-2 md:py-4 print:hidden"
  >
    <div class="mb-4 hidden size-12 place-items-center rounded-2xl bg-emerald-600 text-xl font-bold text-white md:grid" aria-hidden="true">P</div>

    {#each visible as item (item.path)}
      <a
        href={item.path}
        onclick={(e) => go(e, item.path)}
        aria-current={router.path === item.path ? 'page' : undefined}
        class="flex min-h-14 min-w-20 flex-col items-center justify-center gap-1 rounded-xl px-3 py-2 text-sm font-semibold text-muted transition-colors hover:bg-tint aria-[current=page]:bg-tint aria-[current=page]:text-emerald-700 dark:aria-[current=page]:text-emerald-400"
      >
        <svg viewBox="0 0 24 24" class="size-6" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d={item.icon} /></svg>
        {item.label}
      </a>
    {/each}

    <div class="md:mt-auto md:w-full">
      <button
        type="button"
        onclick={signOut}
        class="flex min-h-14 w-full flex-col items-center justify-center rounded-xl px-3 py-2 text-sm text-muted hover:bg-tint"
      >
        <span class="max-w-full truncate font-semibold text-ink">{session.user?.display_name}</span>
        Sign out
      </button>
    </div>
  </nav>

  <main class="min-h-0 flex-1 overflow-auto print:overflow-visible">
    {@render children()}
  </main>
</div>
