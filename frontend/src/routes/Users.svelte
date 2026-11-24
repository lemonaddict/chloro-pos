<script>
  import { onMount } from 'svelte';
  import { api } from '../lib/api.js';
  import Dialog from '../components/Dialog.svelte';
  import Icon from '../components/Icon.svelte';

  const blank = () => ({ display_name: '', username: '', password: '', max_cashiers: 2 });

  // 14 karakter acak dari CSPRNG browser, tanpa huruf yang mirip (l, I, 1, O, 0).
  function generate() {
    const chars = 'abcdefghjkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789';
    return Array.from(crypto.getRandomValues(new Uint32Array(14)), (n) => chars[n % chars.length]).join('');
  }

  let users = $state([]);
  let error = $state('');
  let notice = $state(null); // password yang baru dibuat, ditampilkan sekali

  let addOpen = $state(false);
  let form = $state(blank());
  let formError = $state('');
  let saving = $state(false);

  let resetOpen = $state(false);
  let resetTarget = $state(null);
  let newPassword = $state('');
  let resetError = $state('');

  let limitOpen = $state(false);
  let limitTarget = $state(null);
  let limitValue = $state(2);
  let limitError = $state('');

  let offOpen = $state(false);
  let offTarget = $state(null);
  let offError = $state('');

  async function load() {
    try {
      users = await api.get('/users');
      error = '';
    } catch (e) {
      error = e.message;
    }
  }
  onMount(load);

  function openAdd() {
    form = { ...blank(), password: generate() };
    formError = '';
    addOpen = true;
  }

  async function create() {
    saving = true;
    formError = '';
    try {
      const made = await api.post('/users', { ...form, max_cashiers: Number(form.max_cashiers) });
      notice = { title: 'Akun pelanggan dibuat', name: made.display_name, username: made.username, password: form.password };
      addOpen = false;
      await load();
    } catch (e) {
      formError = e.message;
    } finally {
      saving = false;
    }
  }

  function openReset(user) {
    resetTarget = user;
    newPassword = generate();
    resetError = '';
    resetOpen = true;
  }

  async function reset() {
    try {
      await api.post(`/users/${resetTarget.id}/password`, { password: newPassword });
      resetOpen = false;
      notice = { title: 'Password baru dibuat', name: resetTarget.display_name, username: resetTarget.username, password: newPassword };
    } catch (e) {
      resetError = e.message;
    }
  }

  function openLimit(user) {
    limitTarget = user;
    limitValue = user.max_cashiers;
    limitError = '';
    limitOpen = true;
  }

  async function saveLimit() {
    try {
      await api.put(`/users/${limitTarget.id}`, {
        display_name: limitTarget.display_name,
        is_active: limitTarget.is_active,
        max_cashiers: Number(limitValue),
      });
      limitOpen = false;
      await load();
    } catch (e) {
      limitError = e.message;
    }
  }

  async function setActive(user, active) {
    try {
      await api.put(`/users/${user.id}`, { display_name: user.display_name, is_active: active, max_cashiers: user.max_cashiers });
      offOpen = false;
      await load();
    } catch (e) {
      if (offOpen) offError = e.message;
      else error = e.message;
    }
  }

  function askDeactivate(user) {
    offTarget = user;
    offError = '';
    offOpen = true;
  }
</script>

<div class="mx-auto max-w-4xl space-y-5 p-4 lg:p-6">
  <div class="flex flex-wrap items-center gap-3">
    <div class="mr-auto min-w-0 basis-64">
      <h1 class="text-2xl font-bold">Pengguna</h1>
      <p class="text-muted">Pelanggan SOHO yang memakai ChloroPOS. Dari sini kamu hanya mengatur akunnya. Stok dan penjualan toko mereka tidak terlihat.</p>
    </div>
    <button type="button" class="btn btn-primary w-full sm:w-auto" onclick={openAdd}><Icon name="plus" /> Tambah pelanggan</button>
  </div>

  {#if notice}
    <div role="status" class="space-y-2 rounded-2xl border border-emerald-300 bg-tint p-5 dark:border-emerald-800">
      <p class="font-semibold">{notice.title}: {notice.name}</p>
      <p class="break-words tabular-nums">Username <span class="select-all font-mono font-semibold">{notice.username}</span> dan password <span class="select-all break-all font-mono font-semibold">{notice.password}</span></p>
      <p class="text-sm text-muted">Catat dan berikan ke pelanggan sekarang. Password ini tidak akan ditampilkan lagi. Kasir tokonya dibuat sendiri oleh pemilik di menu Akun kasir.</p>
      <button type="button" class="btn btn-quiet min-h-11 text-base" onclick={() => (notice = null)}>Sudah dicatat</button>
    </div>
  {/if}

  {#if error}
    <p role="alert" class="rounded-xl bg-red-50 p-4 text-red-800 dark:bg-red-950 dark:text-red-200">{error}</p>
  {:else}
    <ul class="space-y-3">
    {#each users as u (u.id)}
      <li class="rounded-2xl border border-line bg-panel p-4 {u.is_active ? '' : 'text-muted'}">
        <div class="flex flex-wrap items-center gap-x-4 gap-y-2">
          <div class="min-w-0 flex-1 basis-48">
            <p class="truncate text-lg font-semibold">{u.display_name}</p>
            <p class="truncate text-sm text-muted">{u.username}</p>
          </div>
          <p class="tabular-nums"><span class="text-muted">Kasir</span> {u.seats_used} dari {u.max_cashiers}</p>
          <span class="rounded-full px-3 py-1 text-sm font-semibold {u.is_active ? 'bg-emerald-100 text-emerald-900 dark:bg-emerald-950 dark:text-emerald-200' : 'bg-line text-muted'}">{u.is_active ? 'Aktif' : 'Nonaktif'}</span>
        </div>
        <div class="mt-3 grid grid-cols-2 gap-2 sm:flex sm:flex-wrap sm:justify-end">
          <button type="button" class="btn btn-quiet min-h-11 px-3 text-base" onclick={() => openLimit(u)}>Jatah kasir</button>
          <button type="button" class="btn btn-quiet min-h-11 px-3 text-base" onclick={() => openReset(u)}>Reset password</button>
          {#if u.is_active}
            <button type="button" class="btn btn-quiet col-span-2 min-h-11 px-3 text-base sm:col-span-1" onclick={() => askDeactivate(u)}>Nonaktifkan</button>
          {:else}
            <button type="button" class="btn btn-quiet col-span-2 min-h-11 px-3 text-base sm:col-span-1" onclick={() => setActive(u, true)}>Aktifkan</button>
          {/if}
        </div>
      </li>
    {:else}
      <li class="rounded-2xl border border-dashed border-line px-4 py-8 text-center text-muted">Belum ada pelanggan. Tambahkan yang pertama.</li>
    {/each}
  </ul>
  {/if}
</div>

<!-- Tambah pelanggan -->
<Dialog bind:open={addOpen} label="Tambah pelanggan">
  <form
    class="space-y-4 p-6"
    onsubmit={(e) => {
      e.preventDefault();
      create();
    }}
  >
    <h2 class="text-xl font-semibold">Tambah pelanggan</h2>
    <label class="block"><span class="mb-1 block font-medium">Nama usaha</span><input class="input" bind:value={form.display_name} required maxlength="80" placeholder="Misalnya Toko Maju Bersama" /></label>
    <label class="block">
      <span class="mb-1 block font-medium">Username toko</span>
      <input class="input" bind:value={form.username} required minlength="3" maxlength="40" autocapitalize="none" spellcheck="false" placeholder="huruf kecil, misalnya tmb" />
      <span class="mt-1 block text-sm text-muted">Dipakai pemilik untuk masuk, dan oleh kasirnya sebagai username toko.</span>
    </label>
    <div>
      <label for="new-pass" class="mb-1 block font-medium">Password pemilik</label>
      <div class="flex flex-col gap-2 sm:flex-row">
        <input id="new-pass" class="input font-mono" bind:value={form.password} required minlength="12" maxlength="128" autocomplete="off" />
        <button type="button" class="btn btn-quiet px-4 text-base sm:shrink-0" onclick={() => (form.password = generate())}>Buat baru</button>
      </div>
      <p class="mt-1 text-sm text-muted">Minimal 12 karakter. Password otomatis sudah kuat dan mudah dibaca.</p>
    </div>
    <label class="block">
      <span class="mb-1 block font-medium">Jatah kasir</span>
      <input class="input" type="number" min="0" max="100" step="1" bind:value={form.max_cashiers} required />
      <span class="mt-1 block text-sm text-muted">Berapa kasir yang boleh aktif bersamaan. Naikkan kalau pelanggan membayar lebih.</span>
    </label>
    {#if formError}<p role="alert" class="rounded-xl bg-red-50 px-4 py-3 text-red-800 dark:bg-red-950 dark:text-red-200">{formError}</p>{/if}
    <div class="flex gap-3">
      <button type="button" class="btn btn-quiet flex-1" onclick={() => (addOpen = false)}>Batal</button>
      <button type="submit" class="btn btn-primary flex-[2]" disabled={saving}>{saving ? 'Menyimpan…' : 'Buat akun'}</button>
    </div>
  </form>
</Dialog>

<!-- Jatah kasir -->
<Dialog bind:open={limitOpen} label="Jatah kasir">
  <form
    class="space-y-4 p-6"
    onsubmit={(e) => {
      e.preventDefault();
      saveLimit();
    }}
  >
    <div>
      <h2 class="text-xl font-semibold">Jatah kasir {limitTarget?.display_name}</h2>
      <p class="text-muted">Sekarang {limitTarget?.seats_used} kasir aktif. Menurunkan jatah di bawah itu tidak menonaktifkan kasir yang sudah ada, tapi pemilik tidak bisa menambah atau mengaktifkan lagi sampai jumlahnya di bawah jatah.</p>
    </div>
    <input class="input" type="number" min="0" max="100" step="1" bind:value={limitValue} required aria-label="Jatah kasir" />
    {#if limitError}<p role="alert" class="rounded-xl bg-red-50 px-4 py-3 text-red-800 dark:bg-red-950 dark:text-red-200">{limitError}</p>{/if}
    <div class="flex gap-3">
      <button type="button" class="btn btn-quiet flex-1" onclick={() => (limitOpen = false)}>Batal</button>
      <button type="submit" class="btn btn-primary flex-[2]">Simpan</button>
    </div>
  </form>
</Dialog>

<!-- Reset password -->
<Dialog bind:open={resetOpen} label="Reset password">
  <form
    class="space-y-4 p-6"
    onsubmit={(e) => {
      e.preventDefault();
      reset();
    }}
  >
    <div>
      <h2 class="text-xl font-semibold">Reset password</h2>
      <p class="text-muted">{resetTarget?.display_name} akan keluar dari semua perangkat dan harus masuk dengan password baru. Kasirnya tidak terpengaruh.</p>
    </div>
    <div class="flex flex-col gap-2 sm:flex-row">
      <input class="input font-mono" bind:value={newPassword} required minlength="12" maxlength="128" autocomplete="off" aria-label="Password baru" />
      <button type="button" class="btn btn-quiet px-4 text-base sm:shrink-0" onclick={() => (newPassword = generate())}>Buat baru</button>
    </div>
    {#if resetError}<p role="alert" class="rounded-xl bg-red-50 px-4 py-3 text-red-800 dark:bg-red-950 dark:text-red-200">{resetError}</p>{/if}
    <div class="flex gap-3">
      <button type="button" class="btn btn-quiet flex-1" onclick={() => (resetOpen = false)}>Batal</button>
      <button type="submit" class="btn btn-primary flex-[2]">Simpan password</button>
    </div>
  </form>
</Dialog>

<!-- Konfirmasi nonaktifkan -->
<Dialog bind:open={offOpen} label="Nonaktifkan pelanggan">
  <div class="space-y-4 p-6">
    <div>
      <h2 class="text-xl font-semibold">Nonaktifkan {offTarget?.display_name}?</h2>
      <p class="text-muted">Pemilik dan semua kasirnya langsung keluar dari semua perangkat dan tidak bisa masuk lagi. Datanya tetap tersimpan. Kamu bisa mengaktifkannya kembali kapan saja.</p>
    </div>
    {#if offError}<p role="alert" class="rounded-xl bg-red-50 px-4 py-3 text-red-800 dark:bg-red-950 dark:text-red-200">{offError}</p>{/if}
    <div class="flex gap-3">
      <button type="button" class="btn btn-quiet flex-1" onclick={() => (offOpen = false)}>Batal</button>
      <button type="button" class="btn btn-primary flex-[2]" onclick={() => setActive(offTarget, false)}>Nonaktifkan</button>
    </div>
  </div>
</Dialog>
