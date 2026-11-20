<script>
  import { onMount } from 'svelte';
  import { api } from '../lib/api.js';
  import { money, toCents } from '../lib/money.js';
  import Dialog from '../components/Dialog.svelte';

  const blank = () => ({ name: '', sku: '', barcode: '', price: '', category_id: '', low_stock_threshold: 5, opening_stock: 0, is_active: true });

  let items = $state([]);
  let categories = $state([]);
  let search = $state('');
  let error = $state('');

  let formOpen = $state(false);
  let editing = $state(null); // the product being edited, or null for a new one
  let form = $state(blank());
  let formError = $state('');
  let saving = $state(false);

  let stockOpen = $state(false);
  let target = $state(null);
  let delta = $state('');
  let reason = $state('restock');
  let stockError = $state('');

  let categoryName = $state('');
  let categoryError = $state('');

  async function addCategory() {
    const name = categoryName.trim();
    if (!name) return;
    try {
      const created = await api.post('/categories', { name });
      categories = [...categories, created].sort((a, b) => a.name.localeCompare(b.name));
      categoryName = '';
      categoryError = '';
    } catch (e) {
      categoryError = e.message;
    }
  }

  async function load(q) {
    try {
      const params = new URLSearchParams({ limit: '100', include_inactive: 'true' });
      if (q) params.set('search', q);
      items = await api.get(`/products?${params}`);
      error = '';
    } catch (e) {
      error = e.message;
    }
  }

  onMount(async () => {
    categories = await api.get('/categories').catch(() => []);
  });

  $effect(() => {
    const q = search.trim();
    const timer = setTimeout(() => load(q), 150);
    return () => clearTimeout(timer);
  });

  function openForm(product = null) {
    editing = product;
    form = product
      ? {
          name: product.name,
          sku: product.sku,
          barcode: product.barcode ?? '',
          price: (product.price_cents / 100).toFixed(2),
          category_id: product.category_id ?? '',
          low_stock_threshold: product.low_stock_threshold,
          is_active: product.is_active,
        }
      : blank();
    formError = '';
    formOpen = true;
  }

  async function save() {
    saving = true;
    formError = '';
    const body = {
      name: form.name,
      sku: form.sku,
      barcode: form.barcode || null,
      category_id: form.category_id || null,
      price_cents: toCents(form.price),
      low_stock_threshold: Number(form.low_stock_threshold),
      is_active: form.is_active,
    };
    try {
      if (editing) await api.put(`/products/${editing.id}`, body);
      else await api.post('/products', { ...body, opening_stock: Number(form.opening_stock) });
      formOpen = false;
      await load(search.trim());
    } catch (e) {
      formError = e.message;
    } finally {
      saving = false;
    }
  }

  function openStock(product) {
    target = product;
    delta = '';
    reason = 'restock';
    stockError = '';
    stockOpen = true;
  }

  async function adjust() {
    const n = Number.parseInt(delta, 10);
    if (!Number.isInteger(n) || n === 0) {
      stockError = 'Enter a whole number, like 12 or -3.';
      return;
    }
    try {
      await api.post(`/products/${target.id}/stock-adjustments`, { delta: n, reason });
      stockOpen = false;
      await load(search.trim());
    } catch (e) {
      stockError = e.message;
    }
  }
</script>

<div class="mx-auto max-w-5xl space-y-5 p-4 lg:p-6">
  <div class="flex flex-wrap items-center gap-3">
    <h1 class="mr-auto text-2xl font-bold">Stock and prices</h1>
    <label class="min-w-56 flex-1 sm:flex-none">
      <span class="sr-only">Search products</span>
      <input class="input" type="search" placeholder="Search products" bind:value={search} autocomplete="off" />
    </label>
    <button type="button" class="btn btn-primary" onclick={() => openForm()}>Add product</button>
  </div>

  <form
    class="flex flex-wrap items-center gap-3"
    onsubmit={(e) => {
      e.preventDefault();
      addCategory();
    }}
  >
    <label class="min-w-56 flex-1 sm:max-w-xs">
      <span class="sr-only">New category name</span>
      <input class="input" bind:value={categoryName} placeholder="New category, like Drinks" maxlength="60" />
    </label>
    <button type="submit" class="btn btn-quiet">Add category</button>
    {#if categoryError}<span role="alert" class="text-red-800 dark:text-red-200">{categoryError}</span>{/if}
  </form>

  {#if error}
    <p role="alert" class="rounded-xl bg-red-50 p-4 text-red-800 dark:bg-red-950 dark:text-red-200">{error}</p>
  {:else if items.length === 0}
    <p class="rounded-2xl border border-line bg-panel p-8 text-center text-muted">No products yet. Add your first product to start selling.</p>
  {:else}
    <div class="overflow-x-auto rounded-2xl border border-line bg-panel">
      <table class="w-full text-left">
        <thead class="border-b border-line text-sm text-muted">
          <tr>
            <th class="px-4 py-3 font-medium">Product</th>
            <th class="px-4 py-3 text-right font-medium">Price</th>
            <th class="px-4 py-3 text-right font-medium">In stock</th>
            <th class="px-4 py-3"><span class="sr-only">Actions</span></th>
          </tr>
        </thead>
        <tbody class="divide-y divide-line">
          {#each items as p (p.id)}
            <tr class={p.is_active ? '' : 'text-muted'}>
              <td class="px-4 py-3">
                <div class="font-semibold">{p.name}</div>
                <div class="text-sm text-muted">{p.sku}{p.is_active ? '' : ' (hidden from checkout)'}</div>
              </td>
              <td class="px-4 py-3 text-right tabular-nums">{money(p.price_cents)}</td>
              <td class="px-4 py-3 text-right tabular-nums">
                <span class={p.stock_qty <= p.low_stock_threshold ? 'font-semibold text-amber-700 dark:text-amber-300' : ''}>{p.stock_qty}</span>
              </td>
              <td class="px-4 py-3">
                <div class="flex justify-end gap-2">
                  <button type="button" class="btn btn-quiet min-h-11 text-base" onclick={() => openStock(p)}>Adjust stock</button>
                  <button type="button" class="btn btn-quiet min-h-11 text-base" onclick={() => openForm(p)}>Edit</button>
                </div>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</div>

<!-- Add or edit a product -->
<Dialog bind:open={formOpen} label={editing ? 'Edit product' : 'Add product'}>
  <form
    class="space-y-4 p-6"
    onsubmit={(e) => {
      e.preventDefault();
      save();
    }}
  >
    <h2 class="text-xl font-semibold">{editing ? 'Edit product' : 'Add product'}</h2>
    <label class="block"><span class="mb-1 block font-medium">Name</span><input class="input" bind:value={form.name} required maxlength="120" /></label>
    <div class="grid grid-cols-2 gap-3">
      <label class="block"><span class="mb-1 block font-medium">SKU</span><input class="input" bind:value={form.sku} required maxlength="40" /></label>
      <label class="block"><span class="mb-1 block font-medium">Barcode</span><input class="input" bind:value={form.barcode} maxlength="64" /></label>
    </div>
    <div class="grid grid-cols-2 gap-3">
      <label class="block"><span class="mb-1 block font-medium">Price</span><input class="input tabular-nums" inputmode="decimal" bind:value={form.price} required /></label>
      <label class="block">
        <span class="mb-1 block font-medium">Category</span>
        <select class="input" bind:value={form.category_id}>
          <option value="">None</option>
          {#each categories as c (c.id)}<option value={c.id}>{c.name}</option>{/each}
        </select>
      </label>
    </div>
    <div class="grid grid-cols-2 gap-3">
      {#if !editing}
        <label class="block"><span class="mb-1 block font-medium">Starting stock</span><input class="input tabular-nums" inputmode="numeric" bind:value={form.opening_stock} /></label>
      {/if}
      <label class="block"><span class="mb-1 block font-medium">Warn when stock is at or below</span><input class="input tabular-nums" inputmode="numeric" bind:value={form.low_stock_threshold} /></label>
    </div>
    {#if editing}
      <label class="flex min-h-11 items-center gap-3"><input type="checkbox" class="size-5 accent-emerald-600" bind:checked={form.is_active} /><span>Available at checkout</span></label>
    {/if}

    {#if formError}<p role="alert" class="rounded-xl bg-red-50 px-4 py-3 text-red-800 dark:bg-red-950 dark:text-red-200">{formError}</p>{/if}

    <div class="flex gap-3">
      <button type="button" class="btn btn-quiet flex-1" onclick={() => (formOpen = false)}>Cancel</button>
      <button type="submit" class="btn btn-primary flex-[2]" disabled={saving}>{saving ? 'Saving…' : 'Save product'}</button>
    </div>
  </form>
</Dialog>

<!-- Adjust stock -->
<Dialog bind:open={stockOpen} label="Adjust stock">
  <form
    class="space-y-4 p-6"
    onsubmit={(e) => {
      e.preventDefault();
      adjust();
    }}
  >
    <div>
      <h2 class="text-xl font-semibold">Adjust stock</h2>
      <p class="text-muted">{target?.name}: {target?.stock_qty} in stock now</p>
    </div>
    <label class="block">
      <span class="mb-1 block font-medium">Reason</span>
      <select class="input" bind:value={reason}>
        <option value="restock">Restock (add units)</option>
        <option value="correction">Count correction (add or remove)</option>
        <option value="damage">Damaged or lost (remove units)</option>
      </select>
    </label>
    <label class="block">
      <span class="mb-1 block font-medium">Units to add (use a minus sign to remove)</span>
      <input class="input tabular-nums" inputmode="numeric" bind:value={delta} placeholder="12 or -3" required />
    </label>
    {#if stockError}<p role="alert" class="rounded-xl bg-red-50 px-4 py-3 text-red-800 dark:bg-red-950 dark:text-red-200">{stockError}</p>{/if}
    <div class="flex gap-3">
      <button type="button" class="btn btn-quiet flex-1" onclick={() => (stockOpen = false)}>Cancel</button>
      <button type="submit" class="btn btn-primary flex-[2]">Save change</button>
    </div>
  </form>
</Dialog>
