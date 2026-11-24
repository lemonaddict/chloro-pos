<script>
  import { onMount, tick } from 'svelte';
  import { api } from '../lib/api.js';
  import { cart } from '../lib/cart.svelte.js';
  import { settings } from '../lib/settings.svelte.js';
  import { money, toMinor, toInput, cashSuggestions } from '../lib/money.js';
  import { computeTax } from '../lib/tax.js';
  import { percent } from '../lib/format.js';
  import Dialog from '../components/Dialog.svelte';
  import Receipt from '../components/Receipt.svelte';

  const methods = [
    { id: 'cash', label: 'Tunai' },
    { id: 'qris', label: 'QRIS' },
    { id: 'transfer', label: 'Transfer' },
    { id: 'card', label: 'Kartu' },
  ];

  let query = $state('');
  let categoryId = $state('');
  let products = $state([]);
  let categories = $state([]);
  let loadError = $state('');
  let notice = $state('');
  let confirmClear = $state(false);

  let payOpen = $state(false);
  let method = $state('cash');
  let tendered = $state('');
  let paying = $state(false);
  let payError = $state('');
  let receipt = $state(null);
  let receiptOpen = $state(false);

  let searchEl;
  let tenderEl;
  let latest = 0;
  let noticeTimer;
  let clearTimer;

  // The server does the real math at checkout. This keeps the screen in step with it.
  const taxInfo = $derived(computeTax(cart.total, settings));
  const payable = $derived(taxInfo.total);
  const taxed = $derived(settings.tax_enabled && settings.tax_rate_bps > 0 && taxInfo.tax > 0);

  const tenderedMinor = $derived(toMinor(tendered));
  const changeDue = $derived(method === 'cash' ? Math.max(0, tenderedMinor - payable) : 0);
  const canPay = $derived(method !== 'cash' || tenderedMinor >= payable);
  const quickAmounts = $derived(cashSuggestions(payable));
  const methodLabel = $derived(methods.find((m) => m.id === method)?.label);

  // ---- produk ----
  function fetchProducts(search, category) {
    const params = new URLSearchParams({ limit: '60' });
    if (search) params.set('search', search);
    if (category) params.set('category_id', category);
    return api.get(`/products?${params}`);
  }

  async function refresh(search, category) {
    const mine = ++latest; // abaikan jawaban yang datang terlambat
    try {
      const list = await fetchProducts(search, category);
      if (mine !== latest) return;
      products = list;
      loadError = '';
    } catch (e) {
      if (mine === latest) loadError = e.message;
    }
  }

  onMount(async () => {
    categories = await api.get('/categories').catch(() => []);
  });

  // Cari sambil mengetik, setelah jeda singkat.
  $effect(() => {
    const q = query.trim();
    const c = categoryId;
    const timer = setTimeout(() => refresh(q, c), q ? 150 : 0);
    return () => clearTimeout(timer);
  });

  // Scanner barcode mengetik kodenya lalu menekan Enter: produk yang persis cocok langsung masuk.
  async function onSearchKey(event) {
    if (event.key !== 'Enter') return;
    event.preventDefault();
    const q = query.trim();
    if (!q) return;
    try {
      const list = await fetchProducts(q, '');
      const lower = q.toLowerCase();
      const hit = list.find((p) => p.barcode === q || p.sku.toLowerCase() === lower) ?? (list.length === 1 ? list[0] : null);
      if (hit) {
        add(hit);
        query = '';
      } else {
        flash(list.length ? 'Ada beberapa produk yang cocok. Ketuk salah satunya.' : `Tidak ada produk untuk “${q}”.`);
      }
    } catch (e) {
      flash(e.message);
    }
  }

  // ---- keranjang ----
  function flash(message) {
    notice = message;
    clearTimeout(noticeTimer);
    noticeTimer = setTimeout(() => (notice = ''), 3000);
  }

  function add(product) {
    if (!cart.add(product)) {
      flash(product.stock_status === 'out' ? `Stok ${product.name} habis.` : 'Jumlahnya sudah mencapai stok yang ada.');
    }
  }

  function clearCart() {
    if (!confirmClear) {
      confirmClear = true;
      clearTimeout(clearTimer);
      clearTimer = setTimeout(() => (confirmClear = false), 3000);
      return;
    }
    cart.clear();
    confirmClear = false;
  }

  // ---- pembayaran ----
  function openPayment() {
    if (cart.lines.length === 0) {
      flash('Tambah produk dulu untuk memulai penjualan.');
      return;
    }
    method = 'cash';
    tendered = toInput(payable);
    payError = '';
    payOpen = true;
  }

  const setTender = (minor) => (tendered = toInput(minor));

  async function pay() {
    if (!canPay || paying) return;
    paying = true;
    payError = '';
    try {
      // Yang dikirim hanya ID produk dan jumlah. Server yang menentukan harga, pajak, dan kasirnya.
      receipt = await api.post('/orders', {
        items: cart.lines.map((l) => ({ product_id: l.id, quantity: l.qty })),
        payment_method: method,
        amount_tendered_cents: method === 'cash' ? tenderedMinor : payable,
      });
      cart.clear();
      payOpen = false;
      receiptOpen = true;
      refresh(query.trim(), categoryId); // tampilkan status stok terbaru
    } catch (e) {
      payError = e.message;
      if (e.code === 'insufficient_stock' || e.code === 'product_unavailable') refresh(query.trim(), categoryId);
    } finally {
      paying = false;
    }
  }

  // ---- keyboard: F1 bayar, F2 cari ----
  function onKey(event) {
    if (event.key === 'F1' || event.key === 'F2') event.preventDefault(); // cegah bantuan/rename bawaan browser
    if (payOpen || receiptOpen) return;
    if (event.key === 'F1') openPayment();
    else if (event.key === 'F2') {
      searchEl?.focus();
      searchEl?.select();
    }
  }

  // Kursor siap untuk scanner selama tidak ada dialog. Dilewati di layar sentuh supaya keyboard layar tidak muncul terus.
  $effect(() => {
    if (!payOpen && !receiptOpen && window.matchMedia('(pointer: fine)').matches) {
      tick().then(() => searchEl?.focus());
    }
  });

  $effect(() => {
    if (payOpen && method === 'cash') tick().then(() => tenderEl?.select());
  });

  function stockLabel(p) {
    if (p.stock_status === 'out') return 'Stok habis';
    if (p.stock_status === 'low') return p.stock_qty != null ? `Sisa ${p.stock_qty}` : 'Stok menipis';
    return p.stock_qty != null ? `Stok ${p.stock_qty}` : '';
  }

  function stockClass(p) {
    if (p.stock_status === 'out') return 'font-semibold text-red-700 dark:text-red-300';
    if (p.stock_status === 'low') return 'font-semibold text-amber-700 dark:text-amber-300';
    return 'text-muted';
  }
</script>

<svelte:window onkeydown={onKey} />

<div class="grid h-full grid-cols-1 lg:grid-cols-[minmax(0,1fr)_25rem]">
  <!-- Produk -->
  <section class="flex min-h-0 flex-col gap-4 p-4 lg:p-6" aria-label="Produk">
    <label class="relative block">
      <span class="sr-only">Cari produk</span>
      <input
        bind:this={searchEl}
        bind:value={query}
        onkeydown={onSearchKey}
        type="search"
        autocomplete="off"
        placeholder="Cari nama produk atau pindai barcode"
        class="input pr-16"
      />
      <kbd class="kbd absolute right-3 top-1/2 -translate-y-1/2">F2</kbd>
    </label>

    <div class="flex flex-wrap gap-2" role="group" aria-label="Kategori">
      {#each [{ id: '', name: 'Semua' }, ...categories] as c (c.id)}
        <button type="button" class="chip" aria-pressed={categoryId === c.id} onclick={() => (categoryId = c.id)}>{c.name}</button>
      {/each}
    </div>

    <div class="min-h-0 flex-1 overflow-y-auto">
      {#if loadError}
        <p role="alert" class="rounded-xl bg-red-50 p-4 text-red-800 dark:bg-red-950 dark:text-red-200">{loadError}</p>
      {:else if products.length === 0}
        <p class="p-8 text-center text-muted">Tidak ada produk yang cocok. Coba nama lain atau pindai barcode-nya.</p>
      {:else}
        <ul class="grid grid-cols-2 gap-3 sm:grid-cols-3 xl:grid-cols-4">
          {#each products as p (p.id)}
            <li>
              <button type="button" class="tile" disabled={p.stock_status === 'out'} onclick={() => add(p)}>
                <span class="line-clamp-2 text-lg font-semibold leading-snug">{p.name}</span>
                <span class="mt-auto text-2xl font-bold tabular-nums text-emerald-700 dark:text-emerald-400">{money(p.price_cents)}</span>
                <span class="min-h-5 text-sm {stockClass(p)}">{stockLabel(p)}</span>
              </button>
            </li>
          {/each}
        </ul>
      {/if}
    </div>
  </section>

  <!-- Penjualan saat ini -->
  <aside class="flex min-h-0 flex-col border-t border-line bg-panel lg:border-l lg:border-t-0" aria-label="Penjualan saat ini">
    <header class="flex items-center justify-between px-5 py-4">
      <h2 class="text-lg font-semibold">Penjualan saat ini</h2>
      {#if cart.lines.length > 0}
        <button type="button" class="min-h-11 px-2 text-sm font-semibold text-muted underline-offset-4 hover:underline" onclick={clearCart}>
          {confirmClear ? 'Ketuk lagi untuk kosongkan' : 'Kosongkan'}
        </button>
      {/if}
    </header>

    <div class="min-h-40 flex-1 overflow-y-auto px-5">
      {#if cart.lines.length === 0}
        <p class="py-10 text-center text-muted">Ketuk produk atau pindai barcode untuk memulai penjualan.</p>
      {:else}
        <ul class="divide-y divide-line">
          {#each cart.lines as l (l.id)}
            <li class="py-3">
              <div class="flex items-baseline justify-between gap-3">
                <span class="font-semibold">{l.name}</span>
                <span class="font-semibold tabular-nums">{money(l.qty * l.price_cents)}</span>
              </div>
              <div class="mt-2 flex items-center gap-2">
                <button type="button" class="step" aria-label="Kurangi satu {l.name}" onclick={() => cart.change(l.id, -1)}>−</button>
                <span class="w-10 text-center text-lg font-semibold tabular-nums">{l.qty}</span>
                <button type="button" class="step" aria-label="Tambah satu {l.name}" onclick={() => cart.change(l.id, 1)}>+</button>
                <span class="ml-auto text-sm tabular-nums text-muted">{money(l.price_cents)} / pcs</span>
              </div>
            </li>
          {/each}
        </ul>
      {/if}
    </div>

    <footer class="space-y-3 bg-tint p-5">
      <p class="min-h-6 text-sm font-medium text-amber-700 dark:text-amber-300" role="status">{notice}</p>
      {#if taxed}
        {#if settings.tax_inclusive}
          <p class="text-sm text-muted">Sudah termasuk {settings.tax_label} {percent(settings.tax_rate_bps)}%: <span class="tabular-nums">{money(taxInfo.tax)}</span></p>
        {:else}
          <div class="space-y-1 text-muted">
            <p class="flex justify-between"><span>Subtotal</span><span class="tabular-nums">{money(cart.total)}</span></p>
            <p class="flex justify-between"><span>{settings.tax_label} {percent(settings.tax_rate_bps)}%</span><span class="tabular-nums">{money(taxInfo.tax)}</span></p>
          </div>
        {/if}
      {/if}
      <div class="flex items-baseline justify-between">
        <span class="text-lg text-muted">Total</span>
        <span class="text-4xl font-bold tabular-nums">{money(payable)}</span>
      </div>
      <button type="button" class="btn btn-primary h-16 w-full text-2xl" disabled={cart.lines.length === 0} onclick={openPayment}>
        Bayar <kbd class="kbd-on-primary">F1</kbd>
      </button>
    </footer>
  </aside>
</div>

<!-- Pembayaran -->
<Dialog bind:open={payOpen} label="Pembayaran">
  <form
    class="space-y-5 p-6"
    onsubmit={(e) => {
      e.preventDefault();
      pay();
    }}
  >
    <div class="flex items-baseline justify-between gap-4">
      <h2 class="text-xl font-semibold">Pembayaran</h2>
      <span class="text-3xl font-bold tabular-nums">{money(payable)}</span>
    </div>

    <div class="grid grid-cols-2 gap-2 sm:grid-cols-4" role="radiogroup" aria-label="Metode pembayaran">
      {#each methods as m (m.id)}
        <button type="button" role="radio" aria-checked={method === m.id} class="chip justify-center py-3" onclick={() => (method = m.id)}>{m.label}</button>
      {/each}
    </div>

    {#if method === 'cash'}
      <div class="space-y-3">
        <label class="block">
          <span class="mb-1 block font-medium">Uang diterima (Rp)</span>
          <input bind:this={tenderEl} bind:value={tendered} inputmode="numeric" autocomplete="off" class="input text-2xl tabular-nums" />
        </label>
        <div class="flex flex-wrap gap-2">
          <button type="button" class="chip" onclick={() => setTender(payable)}>Uang pas</button>
          {#each quickAmounts as amount (amount)}
            <button type="button" class="chip tabular-nums" onclick={() => setTender(amount)}>{money(amount)}</button>
          {/each}
        </div>
        <p class="flex items-baseline justify-between rounded-xl bg-tint px-4 py-3">
          <span class="font-medium">Kembalian</span>
          <span class="text-3xl font-bold tabular-nums text-emerald-700 dark:text-emerald-400">{money(changeDue)}</span>
        </p>
      </div>
    {:else}
      <p class="rounded-xl bg-tint px-4 py-3 text-muted">Terima {money(payable)} lewat {methodLabel}, lalu selesaikan penjualan.</p>
    {/if}

    {#if payError}
      <p role="alert" class="rounded-xl bg-red-50 px-4 py-3 text-red-800 dark:bg-red-950 dark:text-red-200">{payError}</p>
    {/if}

    <div class="flex gap-3">
      <button type="button" class="btn btn-quiet flex-1" onclick={() => (payOpen = false)}>Kembali</button>
      <button type="submit" class="btn btn-primary flex-[2]" disabled={!canPay || paying}>{paying ? 'Menyimpan…' : 'Selesaikan penjualan'}</button>
    </div>
  </form>
</Dialog>

<!-- Struk -->
<Dialog bind:open={receiptOpen} label="Struk">
  {#if receipt}
    <Receipt order={receipt} onnew={() => (receiptOpen = false)} />
  {/if}
</Dialog>
