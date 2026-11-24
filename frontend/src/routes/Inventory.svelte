<script>
  import { onMount } from 'svelte';
  import { api } from '../lib/api.js';
  import { money, toMinor, toInput } from '../lib/money.js';
  import Dialog from '../components/Dialog.svelte';

  const blank = () => ({ name: '', sku: '', barcode: '', price: '', category_id: '', low_stock_threshold: 5, opening_stock: 0, is_active: true });

  let items = $state([]);
  let categories = $state([]);
  let search = $state('');
  let error = $state('');

  let formOpen = $state(false);
  let editing = $state(null); // produk yang diubah, atau null untuk produk baru
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
      categories = [...categories, created].sort((a, b) => a.name.localeCompare(b.name, 'id'));
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
          price: toInput(product.price_cents),
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
      price_cents: toMinor(form.price),
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
      stockError = 'Isi dengan bilangan bulat, misalnya 12 atau -3.';
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
    <h1 class="mr-auto text-2xl font-bold">Stok dan harga</h1>
    <label class="min-w-56 flex-1 sm:flex-none">
      <span class="sr-only">Cari produk</span>
      <input class="input" type="search" placeholder="Cari produk" bind:value={search} autocomplete="off" />
    </label>
    <button type="button" class="btn btn-primary" onclick={() => openForm()}>Tambah produk</button>
  </div>

  <form
    class="flex flex-wrap items-center gap-3"
    onsubmit={(e) => {
      e.preventDefault();
      addCategory();
    }}
  >
    <label class="min-w-56 flex-1 sm:max-w-xs">
      <span class="sr-only">Nama kategori baru</span>
      <input class="input" bind:value={categoryName} placeholder="Kategori baru, misalnya Minuman" maxlength="60" />
    </label>
    <button type="submit" class="btn btn-quiet">Tambah kategori</button>
    {#if categoryError}<span role="alert" class="text-red-800 dark:text-red-200">{categoryError}</span>{/if}
  </form>

  {#if error}
    <p role="alert" class="rounded-xl bg-red-50 p-4 text-red-800 dark:bg-red-950 dark:text-red-200">{error}</p>
  {:else if items.length === 0}
    <p class="rounded-2xl border border-line bg-panel p-8 text-center text-muted">Belum ada produk. Tambahkan produk pertamamu untuk mulai berjualan.</p>
  {:else}
    <div class="overflow-x-auto rounded-2xl border border-line bg-panel">
      <table class="w-full text-left">
        <thead class="border-b border-line text-sm text-muted">
          <tr>
            <th class="px-4 py-3 font-medium">Produk</th>
            <th class="px-4 py-3 text-right font-medium">Harga</th>
            <th class="px-4 py-3 text-right font-medium">Stok</th>
            <th class="px-4 py-3"><span class="sr-only">Aksi</span></th>
          </tr>
        </thead>
        <tbody class="divide-y divide-line">
          {#each items as p (p.id)}
            <tr class={p.is_active ? '' : 'text-muted'}>
              <td class="px-4 py-3">
                <div class="font-semibold">{p.name}</div>
                <div class="text-sm text-muted">{p.sku}{p.is_active ? '' : ' (tidak tampil di kasir)'}</div>
              </td>
              <td class="px-4 py-3 text-right tabular-nums">{money(p.price_cents)}</td>
              <td class="px-4 py-3 text-right tabular-nums">
                <span class={p.stock_status !== 'ok' ? 'font-semibold text-amber-700 dark:text-amber-300' : ''}>{p.stock_qty}</span>
              </td>
              <td class="px-4 py-3">
                <div class="flex justify-end gap-2">
                  <button type="button" class="btn btn-quiet min-h-11 text-base" onclick={() => openStock(p)}>Atur stok</button>
                  <button type="button" class="btn btn-quiet min-h-11 text-base" onclick={() => openForm(p)}>Ubah</button>
                </div>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</div>

<!-- Tambah atau ubah produk -->
<Dialog bind:open={formOpen} label={editing ? 'Ubah produk' : 'Tambah produk'}>
  <form
    class="space-y-4 p-6"
    onsubmit={(e) => {
      e.preventDefault();
      save();
    }}
  >
    <h2 class="text-xl font-semibold">{editing ? 'Ubah produk' : 'Tambah produk'}</h2>
    <label class="block"><span class="mb-1 block font-medium">Nama produk</span><input class="input" bind:value={form.name} required maxlength="120" /></label>
    <div class="grid grid-cols-2 gap-3">
      <label class="block"><span class="mb-1 block font-medium">SKU</span><input class="input" bind:value={form.sku} required maxlength="40" /></label>
      <label class="block"><span class="mb-1 block font-medium">Barcode</span><input class="input" bind:value={form.barcode} maxlength="64" /></label>
    </div>
    <div class="grid grid-cols-2 gap-3">
      <label class="block"><span class="mb-1 block font-medium">Harga (Rp)</span><input class="input tabular-nums" inputmode="numeric" bind:value={form.price} placeholder="15000" required /></label>
      <label class="block">
        <span class="mb-1 block font-medium">Kategori</span>
        <select class="input" bind:value={form.category_id}>
          <option value="">Tanpa kategori</option>
          {#each categories as c (c.id)}<option value={c.id}>{c.name}</option>{/each}
        </select>
      </label>
    </div>
    <div class="grid grid-cols-2 gap-3">
      {#if !editing}
        <label class="block"><span class="mb-1 block font-medium">Stok awal</span><input class="input tabular-nums" inputmode="numeric" bind:value={form.opening_stock} /></label>
      {/if}
      <label class="block"><span class="mb-1 block font-medium">Beri peringatan saat stok ≤</span><input class="input tabular-nums" inputmode="numeric" bind:value={form.low_stock_threshold} /></label>
    </div>
    {#if editing}
      <label class="flex min-h-11 items-center gap-3"><input type="checkbox" class="size-5 accent-emerald-600" bind:checked={form.is_active} /><span>Tampil di kasir</span></label>
    {/if}

    {#if formError}<p role="alert" class="rounded-xl bg-red-50 px-4 py-3 text-red-800 dark:bg-red-950 dark:text-red-200">{formError}</p>{/if}

    <div class="flex gap-3">
      <button type="button" class="btn btn-quiet flex-1" onclick={() => (formOpen = false)}>Batal</button>
      <button type="submit" class="btn btn-primary flex-[2]" disabled={saving}>{saving ? 'Menyimpan…' : 'Simpan produk'}</button>
    </div>
  </form>
</Dialog>

<!-- Atur stok -->
<Dialog bind:open={stockOpen} label="Atur stok">
  <form
    class="space-y-4 p-6"
    onsubmit={(e) => {
      e.preventDefault();
      adjust();
    }}
  >
    <div>
      <h2 class="text-xl font-semibold">Atur stok</h2>
      <p class="text-muted">{target?.name}: stok sekarang {target?.stock_qty}</p>
    </div>
    <label class="block">
      <span class="mb-1 block font-medium">Alasan</span>
      <select class="input" bind:value={reason}>
        <option value="restock">Restock (menambah)</option>
        <option value="correction">Koreksi hitungan (tambah atau kurang)</option>
        <option value="damage">Rusak atau hilang (mengurangi)</option>
      </select>
    </label>
    <label class="block">
      <span class="mb-1 block font-medium">Jumlah (pakai tanda minus untuk mengurangi)</span>
      <input class="input tabular-nums" inputmode="numeric" bind:value={delta} placeholder="12 atau -3" required />
    </label>
    {#if stockError}<p role="alert" class="rounded-xl bg-red-50 px-4 py-3 text-red-800 dark:bg-red-950 dark:text-red-200">{stockError}</p>{/if}
    <div class="flex gap-3">
      <button type="button" class="btn btn-quiet flex-1" onclick={() => (stockOpen = false)}>Batal</button>
      <button type="submit" class="btn btn-primary flex-[2]">Simpan perubahan</button>
    </div>
  </form>
</Dialog>
