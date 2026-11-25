<script>
  import { session, signOut } from '../lib/session.svelte.js';
  import { settings } from '../lib/settings.svelte.js';
  import { router, navigate } from '../lib/router.svelte.js';
  import Logo from './Logo.svelte';
  import Icon from './Icon.svelte';

  let { children } = $props();

  // Mobile bottom nav: 4 items. Desktop sidebar also gets Pengaturan (desktopOnly).
  const ownerItems = [
    { path: '/', label: 'Kasir', icon: 'bag' },
    { path: '/stok', label: 'Stok', icon: 'stock' },
    { path: '/ringkasan', label: 'Ringkasan', icon: 'chart' },
    { path: '/akun-kasir', label: 'Akun kasir', icon: 'users' },
    { path: '/pengaturan', label: 'Pengaturan', icon: 'sliders', desktopOnly: true },
  ];
  // The company's admin manages customers and nothing else, so that is the only page in the menu.
  const adminItems = [{ path: '/pengguna', label: 'Pengguna', icon: 'users' }];

  const role = $derived(session.user?.role);
  const items = $derived(role === 'admin' ? adminItems : ownerItems);
  const hasMenu = $derived(role === 'owner' || role === 'admin');
  // A receipt without an address or phone number isn't a proper nota. Nudge the owner until it's filled in.
  const incomplete = $derived(role === 'owner' && settings.loaded && (!settings.address || !settings.phone));
  const onSettings = $derived(router.path === '/pengaturan');

  function go(event, path) {
    event.preventDefault();
    navigate(path);
  }
</script>

{#if hasMenu}
  <!--
    fixed inset-0 locks the shell to the viewport so the document never scrolls.
    Only <main> scrolls → no double scrollbar. print: reverts to normal flow.
  -->
  <div class="fixed inset-0 flex flex-col overflow-hidden md:flex-row print:static print:block print:h-auto print:overflow-visible">
    <nav
      aria-label="Menu utama"
      class="order-last flex shrink-0 items-center justify-around gap-1 border-t border-line bg-panel p-1.5 md:order-first md:w-28 md:flex-col md:justify-start md:gap-2 md:border-r md:border-t-0 md:px-2 md:py-4 print:hidden"
    >
      <div class="mb-4 hidden md:block"><Logo size="size-12" /></div>

      {#each items as item (item.path)}
        <a
          href={item.path}
          onclick={(e) => go(e, item.path)}
          aria-current={router.path === item.path ? 'page' : undefined}
          class="flex min-h-14 min-w-16 flex-1 flex-col items-center justify-center gap-1 rounded-xl px-2 py-2 text-xs font-semibold text-muted transition-colors hover:bg-tint aria-[current=page]:bg-tint aria-[current=page]:text-emerald-700 md:w-full md:flex-none md:text-sm dark:aria-[current=page]:text-emerald-400 {item.desktopOnly ? 'hidden md:flex' : ''}"
        >
          <Icon name={item.icon} class="size-6" />
          {item.label}
        </a>
      {/each}

      <div class="mt-auto hidden w-full md:block">
        <p class="truncate px-1 pb-1 text-center text-xs font-semibold text-ink">{session.user?.display_name}</p>
      </div>
    </nav>

    <div class="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
      <!-- Header: on mobile, settings gear next to Keluar. On desktop, settings is in the sidebar. -->
      <header class="flex shrink-0 items-center gap-2 border-b border-line bg-panel px-4 py-2.5 print:hidden sm:gap-3">
        <div class="min-w-0 flex-1">
          <p class="truncate text-sm font-semibold text-ink md:text-base">{settings.business_name || 'ChloroPOS'}</p>
          <p class="truncate text-xs text-muted">Halo, {session.user?.display_name}</p>
        </div>
        {#if role === 'owner'}
          <button
            type="button"
            onclick={() => navigate('/pengaturan')}
            aria-label="Pengaturan"
            aria-current={onSettings ? 'page' : undefined}
            title="Pengaturan"
            class="grid size-10 shrink-0 place-items-center rounded-full border border-line text-muted transition-colors hover:bg-tint hover:text-ink aria-[current=page]:border-emerald-600 aria-[current=page]:bg-tint aria-[current=page]:text-emerald-700 md:hidden dark:aria-[current=page]:text-emerald-400"
          >
            <Icon name="sliders" class="size-5" />
          </button>
        {/if}
        <button
          type="button"
          onclick={signOut}
          class="inline-flex shrink-0 items-center gap-1.5 rounded-full border border-line bg-panel px-3 py-1.5 text-xs font-semibold text-ink hover:bg-tint md:text-sm"
        >
          <Icon name="logout" class="size-3.5 md:size-4" />
          Keluar
        </button>
      </header>

      <main class="min-h-0 flex-1 overflow-y-auto overscroll-contain print:overflow-visible">
        {#if incomplete}
          <div class="flex flex-wrap items-center gap-3 border-b border-amber-300 bg-amber-50 px-4 py-3 text-amber-950 print:hidden dark:border-amber-800 dark:bg-amber-950 dark:text-amber-100">
            <Icon name="alert" class="size-5 shrink-0" />
            <p class="flex-1">Alamat dan nomor telepon usaha belum diisi, jadi nota belum lengkap.</p>
            <button type="button" class="btn btn-quiet min-h-10 px-4 text-base" onclick={() => navigate('/pengaturan')}>Lengkapi sekarang</button>
          </div>
        {/if}
        {@render children()}
      </main>
    </div>
  </div>
{:else}
  <!-- A kasir gets the till and nothing else: no menu, no totals, no stock counts. -->
  <div class="fixed inset-0 flex flex-col overflow-hidden print:static print:block print:h-auto print:overflow-visible">
    <header class="flex shrink-0 items-center gap-3 border-b border-line bg-panel px-4 py-2.5 print:hidden">
      <Logo size="size-9" />
      <span class="min-w-0 truncate font-semibold">{settings.business_name}</span>
      <span class="ml-auto hidden truncate text-sm text-muted sm:block">{session.user?.display_name}</span>
      <button type="button" class="chip gap-2" onclick={signOut}><Icon name="logout" class="size-4" /> Keluar</button>
    </header>
    <main class="min-h-0 flex-1 overflow-y-auto overscroll-contain print:overflow-visible">{@render children()}</main>
  </div>
{/if}
