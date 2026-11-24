<script>
  import { api } from '../lib/api.js';
  import { settings, applySettings } from '../lib/settings.svelte.js';
  import { computeTax } from '../lib/tax.js';
  import { formatNpwp, percent } from '../lib/format.js';
  import ReceiptPaper from '../components/ReceiptPaper.svelte';
  import Icon from '../components/Icon.svelte';
  import { theme, setTheme } from '../lib/theme.svelte.js';
  import { session, signOut } from '../lib/session.svelte.js';

  const fromSettings = () => ({
    business_name: settings.business_name,
    legal_name: settings.legal_name,
    address: settings.address,
    phone: settings.phone,
    email: settings.email,
    npwp: settings.npwp ? formatNpwp(settings.npwp) : '',
    receipt_footer: settings.receipt_footer,
    tax_enabled: settings.tax_enabled,
    tax_label: settings.tax_label,
    tax_pct: percent(settings.tax_rate_bps),
    tax_inclusive: settings.tax_inclusive,
  });

  let form = $state(fromSettings());
  let dirty = false;
  let saving = $state(false);
  let error = $state('');
  let saved = $state(false);
  let savedTimer;

  // Kalau halaman dibuka sebelum pengaturan selesai dimuat, isi formulir begitu datanya ada.
  $effect(() => {
    const fresh = fromSettings();
    if (!dirty) form = fresh;
  });

  const bps = $derived.by(() => {
    const n = Number(String(form.tax_pct).replace(',', '.'));
    return Number.isFinite(n) ? Math.round(n * 100) : 0;
  });

  // Struk contoh di sebelah kanan ikut berubah saat formulir diubah. Angkanya: 36.000 + 18.000 + 17.000 = 71.000.
  const sampleItems = [
    { product_id: 'a', product_name: 'Kopi susu', quantity: 2, unit_price_cents: 1_800_000, line_total_cents: 3_600_000 },
    { product_id: 'b', product_name: 'Roti bakar cokelat', quantity: 1, unit_price_cents: 1_800_000, line_total_cents: 1_800_000 },
    { product_id: 'c', product_name: 'Gula pasir 1 kg', quantity: 1, unit_price_cents: 1_700_000, line_total_cents: 1_700_000 },
  ];

  const previewBiz = $derived({
    business_name: form.business_name || 'Nama Usaha',
    legal_name: form.legal_name,
    address: form.address,
    phone: form.phone,
    email: form.email,
    npwp: form.npwp.replace(/\D/g, ''),
    receipt_footer: form.receipt_footer,
  });

  const previewOrder = $derived.by(() => {
    const subtotal = 7_100_000;
    const { tax, total } = computeTax(subtotal, { tax_enabled: form.tax_enabled, tax_rate_bps: bps, tax_inclusive: form.tax_inclusive });
    const on = form.tax_enabled && bps > 0 && tax > 0;
    const paid = Math.ceil(total / 10_000_000) * 10_000_000;
    return {
      receipt_no: 123,
      created_at: new Date().toISOString(),
      cashier_name: 'Nama Kasir',
      items: sampleItems,
      subtotal_cents: subtotal,
      tax_cents: on ? tax : 0,
      tax_rate_bps: on ? bps : 0,
      tax_inclusive: on ? form.tax_inclusive : false,
      tax_label: on ? form.tax_label : '',
      total_cents: total,
      payment_method: 'cash',
      amount_tendered_cents: paid,
      change_cents: paid - total,
    };
  });

  async function save(event) {
    event.preventDefault();
    saving = true;
    error = '';
    try {
      const result = await api.put('/settings', {
        business_name: form.business_name,
        legal_name: form.legal_name,
        address: form.address,
        phone: form.phone,
        email: form.email,
        npwp: form.npwp,
        receipt_footer: form.receipt_footer,
        tax_enabled: form.tax_enabled,
        tax_label: form.tax_label,
        tax_rate_bps: bps,
        tax_inclusive: form.tax_inclusive,
      });
      applySettings(result);
      dirty = false;
      form = fromSettings();
      saved = true;
      clearTimeout(savedTimer);
      savedTimer = setTimeout(() => (saved = false), 4000);
    } catch (e) {
      error = e.message;
    } finally {
      saving = false;
    }
  }
</script>

<div class="mx-auto max-w-5xl p-4 lg:p-6">
  <h1 class="text-2xl font-bold">Pengaturan usaha</h1>
  <p class="mt-1 max-w-2xl text-muted">Data ini dicetak di setiap struk. Isi sesuai dokumen resmi usahamu. Untuk tarif pajak dan kewajiban nota atau faktur, tanyakan ke konsultan pajak.</p>

  <section class="mt-6 space-y-3" aria-label="Tampilan">
    <h2 class="text-lg font-semibold">Tampilan</h2>
    <p class="text-sm text-muted">Pilih tema terang, gelap, atau ikut pengaturan perangkat.</p>
    <div class="flex flex-wrap gap-2">
      {#each [
        { id: 'system', label: 'Sistem', icon: 'monitor' },
        { id: 'light', label: 'Terang', icon: 'sun' },
        { id: 'dark', label: 'Gelap', icon: 'moon' },
      ] as opt}
        <button
          type="button"
          class="chip gap-2"
          aria-pressed={theme.preference === opt.id}
          onclick={() => setTheme(opt.id)}
        >
          <Icon name={opt.icon} class="size-4" />
          {opt.label}
        </button>
      {/each}
    </div>
  </section>

  <section class="mt-6 space-y-3" aria-label="Akun">
    <h2 class="text-lg font-semibold">Akun</h2>
    <div class="flex flex-wrap items-center gap-3 rounded-2xl border border-line bg-panel px-4 py-3">
      <div class="min-w-0 flex-1">
        <p class="truncate font-semibold">{session.user?.display_name}</p>
        <p class="text-sm text-muted">Owner</p>
      </div>
      <button
        type="button"
        onclick={signOut}
        class="inline-flex shrink-0 items-center gap-1.5 rounded-full border border-line bg-panel px-3 py-1.5 text-sm font-semibold text-ink hover:bg-tint"
      >
        <Icon name="logout" class="size-4" />
        Keluar
      </button>
    </div>
  </section>

  <div class="mt-6 grid gap-8 lg:grid-cols-[minmax(0,1fr)_22rem]">
    <form onsubmit={save} oninput={() => (dirty = true)} class="space-y-8">
      <section class="space-y-4">
        <h2 class="text-lg font-semibold">Profil usaha</h2>
        <label class="block"><span class="mb-1 block font-medium">Nama usaha</span><input class="input" bind:value={form.business_name} required maxlength="80" /></label>
        <label class="block">
          <span class="mb-1 block font-medium">Nama badan usaha (kalau ada)</span>
          <input class="input" bind:value={form.legal_name} maxlength="120" placeholder="Misalnya CV Maju Jaya" />
        </label>
        <label class="block"><span class="mb-1 block font-medium">Alamat</span><textarea class="input py-3" rows="3" bind:value={form.address} maxlength="300" placeholder="Jalan, nomor, kelurahan, kota"></textarea></label>
        <div class="grid gap-4 sm:grid-cols-2">
          <label class="block"><span class="mb-1 block font-medium">Nomor telepon</span><input class="input" inputmode="tel" bind:value={form.phone} maxlength="30" placeholder="0812 3456 7890" /></label>
          <label class="block"><span class="mb-1 block font-medium">Email</span><input class="input" type="email" bind:value={form.email} maxlength="120" /></label>
        </div>
        <label class="block">
          <span class="mb-1 block font-medium">NPWP (kalau ada)</span>
          <input class="input tabular-nums" inputmode="numeric" bind:value={form.npwp} maxlength="30" placeholder="15 atau 16 angka" />
        </label>
      </section>

      <section class="space-y-4">
        <h2 class="text-lg font-semibold">Pajak</h2>
        <label class="flex min-h-11 items-center gap-3">
          <input type="checkbox" class="size-5 accent-emerald-600" bind:checked={form.tax_enabled} />
          <span>Kenakan pajak di setiap penjualan</span>
        </label>
        {#if form.tax_enabled}
          <div class="grid gap-4 sm:grid-cols-2">
            <label class="block"><span class="mb-1 block font-medium">Nama pajak</span><input class="input" bind:value={form.tax_label} maxlength="20" required /></label>
            <label class="block"><span class="mb-1 block font-medium">Tarif (%)</span><input class="input tabular-nums" inputmode="decimal" bind:value={form.tax_pct} required /></label>
          </div>
          <fieldset class="space-y-2">
            <legend class="mb-1 font-medium">Cara menghitung</legend>
            <label class="flex min-h-11 items-start gap-3"><input type="radio" name="mode" class="mt-1.5 size-5 accent-emerald-600" value={true} bind:group={form.tax_inclusive} /><span>Harga di kasir sudah termasuk pajak<br /><span class="text-sm text-muted">Total tetap sama dengan harga. Struk mencantumkan pajak yang ada di dalamnya.</span></span></label>
            <label class="flex min-h-11 items-start gap-3"><input type="radio" name="mode" class="mt-1.5 size-5 accent-emerald-600" value={false} bind:group={form.tax_inclusive} /><span>Pajak ditambahkan di atas harga<br /><span class="text-sm text-muted">Struk menampilkan subtotal, pajak, lalu total.</span></span></label>
          </fieldset>
        {/if}
        <p class="text-sm text-muted">Pajak dihitung dan dibulatkan ke rupiah penuh oleh server, lalu tersimpan di setiap transaksi. Mengubah tarif tidak mengubah transaksi lama.</p>
      </section>

      <section class="space-y-4">
        <h2 class="text-lg font-semibold">Pesan di bawah struk</h2>
        <label class="block"><span class="sr-only">Pesan di bawah struk</span><textarea class="input py-3" rows="2" bind:value={form.receipt_footer} maxlength="200" placeholder="Terima kasih sudah berbelanja!"></textarea></label>
      </section>

      {#if error}<p role="alert" class="rounded-xl bg-red-50 px-4 py-3 text-red-800 dark:bg-red-950 dark:text-red-200">{error}</p>{/if}
      {#if saved}<p role="status" class="rounded-xl bg-tint px-4 py-3 font-medium text-emerald-800 dark:text-emerald-300">Pengaturan tersimpan.</p>{/if}

      <button type="submit" class="btn btn-primary w-full sm:w-auto" disabled={saving}>{saving ? 'Menyimpan…' : 'Simpan pengaturan'}</button>
    </form>

    <aside class="lg:sticky lg:top-6 lg:self-start" aria-label="Contoh struk">
      <p class="mb-2 font-medium text-muted">Contoh struk</p>
      <div class="rounded-2xl border border-line bg-paper p-5 text-forest shadow-sm">
        <ReceiptPaper order={previewOrder} biz={previewBiz} printable={false} />
      </div>
    </aside>
  </div>
</div>
