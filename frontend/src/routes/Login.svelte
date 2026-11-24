<script>
  import { onMount } from 'svelte';
  import { signIn, signInCashier } from '../lib/session.svelte.js';
  import { money } from '../lib/money.js';
  import Icon from '../components/Icon.svelte';
  import Logo from '../components/Logo.svelte';

  const day = new Date().toLocaleDateString('id-ID', { day: 'numeric', month: 'short' });

  // Contoh penjualan di struk sebelah kiri. Angkanya sengaja pas: 36.000 + 18.000 + 17.000 = 71.000.
  const sale = [
    { qty: 2, name: 'Kopi susu', price: 36000 },
    { qty: 1, name: 'Roti bakar cokelat', price: 18000 },
    { qty: 1, name: 'Gula pasir 1 kg', price: 17000 },
  ];
  const rp = (n) => money(n * 100);

  const perks = [
    { icon: 'bolt', text: 'Bayar sekali ketuk, atau tekan F1' },
    { icon: 'stock', text: 'Stok berkurang otomatis di setiap penjualan' },
    { icon: 'chart', text: 'Total hari ini siap saat tutup kasir' },
  ];

  // 'owner': username + kata sandi (pemilik toko, atau admin). 'cashier': username toko + kode kasir.
  let mode = $state('owner');
  let username = $state('');
  let password = $state('');
  let code = $state('');
  let show = $state(false);
  let capsOn = $state(false);
  let busy = $state(false);
  let error = $state('');
  let userEl;

  const shop = 'Toko Anda';

  onMount(() => userEl?.focus());

  const checkCaps = (e) => (capsOn = e.getModifierState?.('CapsLock') ?? false);

  function setMode(next) {
    mode = next;
    error = '';
    password = '';
    code = '';
    userEl?.focus();
  }

  async function submit(event) {
    event.preventDefault();
    busy = true;
    error = '';
    try {
      if (mode === 'cashier') await signInCashier(username.trim(), code.trim());
      else await signIn(username.trim(), password);
    } catch (e) {
      error = e.message;
      password = '';
      code = '';
    } finally {
      busy = false;
    }
  }
</script>

<main class="grid min-h-dvh bg-page lg:grid-cols-[5fr_6fr]">
  <!-- HP dan tablet kecil: cukup strip merek -->
  <header class="flex items-center gap-3 bg-forest px-6 py-5 text-white lg:hidden">
    <Logo size="size-10" />
    <div>
      <p class="font-semibold leading-tight">ChloroPOS</p>
      <p class="text-sm text-sage-200">{shop}</p>
    </div>
  </header>

  <aside class="hidden flex-col justify-between gap-10 overflow-hidden bg-forest p-12 text-white lg:flex xl:p-16">
    <div class="flex items-center gap-3">
      <Logo size="size-12" class="leaf-in" />
      <div>
        <p class="text-xl font-semibold leading-tight">ChloroPOS</p>
        <p class="text-sm text-sage-200">Cellix Helping Outlet Ready Online POS</p>
      </div>
    </div>

    <!-- Struk yang "tercetak" sekali saat halaman dibuka. Mati otomatis kalau pengguna memilih reduced motion. -->
    <div class="flex justify-center py-2" aria-hidden="true">
      <div class="slip-print w-72 -rotate-2 select-none drop-shadow-2xl">
        <div class="bg-paper px-6 pb-4 pt-6 font-mono text-[13px] leading-6 text-forest">
          <p class="text-center text-base font-bold">{shop}</p>
          <p class="text-center text-sage-500">No. 000128, {day}</p>
          <ul class="my-3 space-y-0.5 border-y border-dashed border-sage-300 py-3">
            {#each sale as line}
              <li class="flex justify-between gap-3">
                <span class="truncate">{line.qty} x {line.name}</span>
                <span class="shrink-0 tabular-nums">{rp(line.price)}</span>
              </li>
            {/each}
          </ul>
          <p class="flex justify-between font-bold"><span>Total</span><span class="tabular-nums">{rp(71000)}</span></p>
          <p class="flex justify-between"><span>Tunai</span><span class="tabular-nums">{rp(100000)}</span></p>
          <p class="flex justify-between"><span>Kembali</span><span class="tabular-nums">{rp(29000)}</span></p>
          <p class="mt-3 text-center">Terima kasih!</p>
        </div>
        <svg class="block h-2.5 w-full" aria-hidden="true">
          <defs>
            <pattern id="slip-teeth" width="14" height="10" patternUnits="userSpaceOnUse">
              <path d="M0 0 L7 10 L14 0 Z" fill="#f7faf7" />
            </pattern>
          </defs>
          <rect width="100%" height="10" fill="url(#slip-teeth)" />
        </svg>
      </div>
    </div>

    <div class="space-y-6">
      <h2 class="max-w-sm text-3xl font-semibold leading-tight tracking-tight">Jualan cepat. Stok tetap jujur.</h2>
      <ul class="space-y-3 text-sage-200">
        {#each perks as perk}
          <li class="flex items-center gap-3">
            <Icon name={perk.icon} class="size-5 shrink-0 text-emerald-300" />
            {perk.text}
          </li>
        {/each}
      </ul>
    </div>
  </aside>

  <section class="flex flex-col justify-center px-6 py-10 sm:px-12 lg:px-16">
    <form onsubmit={submit} class="mx-auto w-full max-w-sm space-y-6 lg:mx-0">
      <div class="space-y-2">
        <h1 class="text-3xl font-semibold tracking-tight">Masuk</h1>
        <p class="text-muted">
          {mode === 'cashier' ? 'Pakai username toko dan kode kasir dari pemilik toko.' : 'Masuk dengan akun usahamu.'}
        </p>
      </div>

      <div class="grid grid-cols-2 gap-2" role="radiogroup" aria-label="Masuk sebagai">
        <button type="button" role="radio" aria-checked={mode === 'owner'} class="chip justify-center py-3" onclick={() => setMode('owner')}>Pemilik</button>
        <button type="button" role="radio" aria-checked={mode === 'cashier'} class="chip justify-center py-3" onclick={() => setMode('cashier')}>Kasir</button>
      </div>

      <div>
        <label for="username" class="mb-1.5 block font-medium">{mode === 'cashier' ? 'Username toko' : 'Username'}</label>
        <div class="relative">
          <Icon name="user" class="pointer-events-none absolute left-4 top-1/2 size-5 -translate-y-1/2 text-muted" />
          <input id="username" bind:this={userEl} bind:value={username} class="input pl-12" autocomplete="username" autocapitalize="none" spellcheck="false" required />
        </div>
      </div>

      {#if mode === 'cashier'}
        <div>
          <label for="code" class="mb-1.5 block font-medium">Kode kasir</label>
          <div class="relative">
            <Icon name="lock" class="pointer-events-none absolute left-4 top-1/2 size-5 -translate-y-1/2 text-muted" />
            <input
              id="code"
              bind:value={code}
              class="input pl-12 font-mono uppercase tracking-widest"
              placeholder="7QK2-M49Q-XD2R"
              autocomplete="off"
              autocapitalize="characters"
              spellcheck="false"
              maxlength="20"
              required
            />
          </div>
          <p class="mt-2 text-sm text-muted">Kode ini khusus untuk satu kasir. Bukan kata sandi pemilik.</p>
        </div>
      {:else}
        <div>
          <label for="password" class="mb-1.5 block font-medium">Kata sandi</label>
          <div class="relative">
            <Icon name="lock" class="pointer-events-none absolute left-4 top-1/2 size-5 -translate-y-1/2 text-muted" />
            <input
              id="password"
              bind:value={password}
              type={show ? 'text' : 'password'}
              class="input px-12"
              autocomplete="current-password"
              onkeydown={checkCaps}
              onkeyup={checkCaps}
              onblur={() => (capsOn = false)}
              required
            />
            <button
              type="button"
              class="absolute right-1 top-1/2 grid size-11 -translate-y-1/2 place-items-center rounded-lg text-muted transition-colors hover:text-ink"
              aria-label={show ? 'Sembunyikan kata sandi' : 'Tampilkan kata sandi'}
              aria-pressed={show}
              onclick={() => (show = !show)}
            >
              <Icon name={show ? 'eyeOff' : 'eye'} />
            </button>
          </div>
          {#if capsOn}
            <p class="mt-2 flex items-center gap-2 text-sm font-medium text-amber-800 dark:text-amber-300">
              <Icon name="alert" class="size-4 shrink-0" /> Caps Lock sedang aktif.
            </p>
          {/if}
        </div>
      {/if}

      {#if error}
        <div role="alert" class="flex gap-3 rounded-xl bg-red-50 px-4 py-3 text-red-800 dark:bg-red-950 dark:text-red-200">
          <Icon name="alert" class="mt-0.5 size-5 shrink-0" />
          <p>{error}</p>
        </div>
      {/if}

      <button type="submit" class="btn btn-primary h-14 w-full text-xl" disabled={busy}>{busy ? 'Sedang masuk…' : 'Masuk'}</button>

      <p class="text-sm text-muted">
        {mode === 'cashier' ? 'Belum punya kode? Minta pemilik toko membuatkannya di menu Akun kasir.' : 'Belum punya akun? Hubungi kami untuk didaftarkan.'}
      </p>
    </form>
  </section>
</main>
