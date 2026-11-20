<script>
  import { onMount, tick } from 'svelte';
  import { api } from '../lib/api.js';
  import { cart } from '../lib/cart.svelte.js';
  import { money, toCents, cashSuggestions } from '../lib/money.js';
  import Dialog from '../components/Dialog.svelte';
  import Receipt from '../components/Receipt.svelte';

  const methods = [
    { id: 'cash', label: 'Cash' },
    { id: 'card', label: 'Card' },
    { id: 'transfer', label: 'Transfer' },
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

  const tenderedCents = $derived(toCents(tendered));
  const changeDue = $derived(method === 'cash' ? Math.max(0, tenderedCents - cart.total) : 0);
  const canPay = $derived(method !== 'cash' || tenderedCents >= cart.total);
  const quickAmounts = $derived(cashSuggestions(cart.total));
  const methodLabel = $derived(methods.find((m) => m.id === method)?.label.toLowerCase());

  // ---- products ----
  function fetchProducts(search, category) {
    const params = new URLSearchParams({ limit: '60' });
    if (search) params.set('search', search);
    if (category) params.set('category_id', category);
    return api.get(`/products?${params}`);
  }

  async function refresh(search, category) {
    const mine = ++latest; // ignore answers that arrive out of order
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

  // Search as you type, after a short pause.
  $effect(() => {
    const q = query.trim();
    const c = categoryId;
    const timer = setTimeout(() => refresh(q, c), q ? 150 : 0);
    return () => clearTimeout(timer);
  });

  // Barcode scanners type the code and press Enter: add the exact match straight away.
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
        flash(list.length ? 'Several products match. Tap one to add it.' : `Nothing found for “${q}”.`);
      }
    } catch (e) {
      flash(e.message);
    }
  }

  // ---- cart ----
  function flash(message) {
    notice = message;
    clearTimeout(noticeTimer);
    noticeTimer = setTimeout(() => (notice = ''), 3000);
  }

  function add(product) {
    if (!cart.add(product)) {
      flash(product.stock_qty > 0 ? `All ${product.stock_qty} in stock are already in the sale.` : `${product.name} is out of stock.`);
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

  // ---- payment ----
  function openPayment() {
    if (cart.lines.length === 0) {
      flash('Add a product to start a sale.');
      return;
    }
    method = 'cash';
    tendered = (cart.total / 100).toFixed(2);
    payError = '';
    payOpen = true;
  }

  function setTender(cents) {
    tendered = (cents / 100).toFixed(2);
  }

  async function pay() {
    if (!canPay || paying) return;
    paying = true;
    payError = '';
    try {
      // Only product IDs and quantities go to the server. It sets the prices and the cashier.
      receipt = await api.post('/orders', {
        items: cart.lines.map((l) => ({ product_id: l.id, quantity: l.qty })),
        payment_method: method,
        amount_tendered_cents: method === 'cash' ? tenderedCents : cart.total,
      });
      cart.clear();
      payOpen = false;
      receiptOpen = true;
      refresh(query.trim(), categoryId); // show the new stock counts
    } catch (e) {
      payError = e.message;
      if (e.code === 'insufficient_stock' || e.code === 'product_unavailable') refresh(query.trim(), categoryId);
    } finally {
      paying = false;
    }
  }

  // ---- keyboard: F1 charge, F2 search ----
  function onKey(event) {
    if (event.key === 'F1' || event.key === 'F2') event.preventDefault(); // keep the browser's help/rename out of the way
    if (payOpen || receiptOpen) return;
    if (event.key === 'F1') openPayment();
    else if (event.key === 'F2') {
      searchEl?.focus();
      searchEl?.select();
    }
  }

  // Keep the cursor ready for a scanner whenever no dialog is open. Skipped on touch screens,
  // where focusing would pop up the on-screen keyboard after every sale.
  $effect(() => {
    if (!payOpen && !receiptOpen && window.matchMedia('(pointer: fine)').matches) {
      tick().then(() => searchEl?.focus());
    }
  });

  $effect(() => {
    if (payOpen && method === 'cash') tick().then(() => tenderEl?.select());
  });

  function stockLabel(p) {
    if (p.stock_qty === 0) return 'Out of stock';
    if (p.stock_qty <= p.low_stock_threshold) return `Only ${p.stock_qty} left`;
    return `${p.stock_qty} in stock`;
  }

  function stockClass(p) {
    if (p.stock_qty === 0) return 'font-semibold text-red-700 dark:text-red-300';
    if (p.stock_qty <= p.low_stock_threshold) return 'font-semibold text-amber-700 dark:text-amber-300';
    return 'text-muted';
  }
</script>

<svelte:window onkeydown={onKey} />

<div class="grid h-full grid-cols-1 lg:grid-cols-[minmax(0,1fr)_25rem]">
  <!-- Products -->
  <section class="flex min-h-0 flex-col gap-4 p-4 lg:p-6" aria-label="Products">
    <label class="relative block">
      <span class="sr-only">Search products</span>
      <input
        bind:this={searchEl}
        bind:value={query}
        onkeydown={onSearchKey}
        type="search"
        autocomplete="off"
        placeholder="Search by name, or scan a barcode"
        class="input pr-16"
      />
      <kbd class="kbd absolute right-3 top-1/2 -translate-y-1/2">F2</kbd>
    </label>

    <div class="flex flex-wrap gap-2" role="group" aria-label="Categories">
      {#each [{ id: '', name: 'All' }, ...categories] as c (c.id)}
        <button type="button" class="chip" aria-pressed={categoryId === c.id} onclick={() => (categoryId = c.id)}>{c.name}</button>
      {/each}
    </div>

    <div class="min-h-0 flex-1 overflow-y-auto">
      {#if loadError}
        <p role="alert" class="rounded-xl bg-red-50 p-4 text-red-800 dark:bg-red-950 dark:text-red-200">{loadError}</p>
      {:else if products.length === 0}
        <p class="p-8 text-center text-muted">No products match. Try another name or scan the barcode.</p>
      {:else}
        <ul class="grid grid-cols-2 gap-3 sm:grid-cols-3 xl:grid-cols-4">
          {#each products as p (p.id)}
            <li>
              <button type="button" class="tile" disabled={p.stock_qty === 0} onclick={() => add(p)}>
                <span class="line-clamp-2 text-lg font-semibold leading-snug">{p.name}</span>
                <span class="mt-auto text-2xl font-bold tabular-nums text-emerald-700 dark:text-emerald-400">{money(p.price_cents)}</span>
                <span class="text-sm {stockClass(p)}">{stockLabel(p)}</span>
              </button>
            </li>
          {/each}
        </ul>
      {/if}
    </div>
  </section>

  <!-- Current sale -->
  <aside class="flex min-h-0 flex-col border-t border-line bg-panel lg:border-l lg:border-t-0" aria-label="Current sale">
    <header class="flex items-center justify-between px-5 py-4">
      <h2 class="text-lg font-semibold">Current sale</h2>
      {#if cart.lines.length > 0}
        <button type="button" class="min-h-11 px-2 text-sm font-semibold text-muted underline-offset-4 hover:underline" onclick={clearCart}>
          {confirmClear ? 'Tap again to clear' : 'Clear'}
        </button>
      {/if}
    </header>

    <div class="min-h-40 flex-1 overflow-y-auto px-5">
      {#if cart.lines.length === 0}
        <p class="py-10 text-center text-muted">Tap a product or scan a barcode to start a sale.</p>
      {:else}
        <ul class="divide-y divide-line">
          {#each cart.lines as l (l.id)}
            <li class="py-3">
              <div class="flex items-baseline justify-between gap-3">
                <span class="font-semibold">{l.name}</span>
                <span class="font-semibold tabular-nums">{money(l.qty * l.price_cents)}</span>
              </div>
              <div class="mt-2 flex items-center gap-2">
                <button type="button" class="step" aria-label="Remove one {l.name}" onclick={() => cart.change(l.id, -1)}>−</button>
                <span class="w-10 text-center text-lg font-semibold tabular-nums">{l.qty}</span>
                <button type="button" class="step" aria-label="Add one {l.name}" onclick={() => cart.change(l.id, 1)}>+</button>
                <span class="ml-auto text-sm tabular-nums text-muted">{money(l.price_cents)} each</span>
              </div>
            </li>
          {/each}
        </ul>
      {/if}
    </div>

    <footer class="space-y-3 bg-tint p-5">
      <p class="min-h-6 text-sm font-medium text-amber-700 dark:text-amber-300" role="status">{notice}</p>
      <div class="flex items-baseline justify-between">
        <span class="text-lg text-muted">Total</span>
        <span class="text-4xl font-bold tabular-nums">{money(cart.total)}</span>
      </div>
      <button type="button" class="btn btn-primary h-16 w-full text-2xl" disabled={cart.lines.length === 0} onclick={openPayment}>
        Charge <kbd class="kbd-on-primary">F1</kbd>
      </button>
    </footer>
  </aside>
</div>

<!-- Payment -->
<Dialog bind:open={payOpen} label="Take payment">
  <form
    class="space-y-5 p-6"
    onsubmit={(e) => {
      e.preventDefault();
      pay();
    }}
  >
    <div class="flex items-baseline justify-between gap-4">
      <h2 class="text-xl font-semibold">Take payment</h2>
      <span class="text-3xl font-bold tabular-nums">{money(cart.total)}</span>
    </div>

    <div class="grid grid-cols-3 gap-2" role="radiogroup" aria-label="Payment method">
      {#each methods as m (m.id)}
        <button type="button" role="radio" aria-checked={method === m.id} class="chip justify-center py-3" onclick={() => (method = m.id)}>{m.label}</button>
      {/each}
    </div>

    {#if method === 'cash'}
      <div class="space-y-3">
        <label class="block">
          <span class="mb-1 block font-medium">Cash received</span>
          <input bind:this={tenderEl} bind:value={tendered} inputmode="decimal" autocomplete="off" class="input text-2xl tabular-nums" />
        </label>
        <div class="flex flex-wrap gap-2">
          <button type="button" class="chip" onclick={() => setTender(cart.total)}>Exact</button>
          {#each quickAmounts as amount (amount)}
            <button type="button" class="chip tabular-nums" onclick={() => setTender(amount)}>{money(amount)}</button>
          {/each}
        </div>
        <p class="flex items-baseline justify-between rounded-xl bg-tint px-4 py-3">
          <span class="font-medium">Change due</span>
          <span class="text-3xl font-bold tabular-nums text-emerald-700 dark:text-emerald-400">{money(changeDue)}</span>
        </p>
      </div>
    {:else}
      <p class="rounded-xl bg-tint px-4 py-3 text-muted">Collect {money(cart.total)} by {methodLabel}, then complete the sale.</p>
    {/if}

    {#if payError}
      <p role="alert" class="rounded-xl bg-red-50 px-4 py-3 text-red-800 dark:bg-red-950 dark:text-red-200">{payError}</p>
    {/if}

    <div class="flex gap-3">
      <button type="button" class="btn btn-quiet flex-1" onclick={() => (payOpen = false)}>Back</button>
      <button type="submit" class="btn btn-primary flex-[2]" disabled={!canPay || paying}>{paying ? 'Saving…' : 'Complete sale'}</button>
    </div>
  </form>
</Dialog>

<!-- Receipt -->
<Dialog bind:open={receiptOpen} label="Receipt">
  {#if receipt}
    <Receipt order={receipt} onnew={() => (receiptOpen = false)} />
  {/if}
</Dialog>
