<script>
  import { onMount } from 'svelte';
  import { api } from '../lib/api.js';
  import { session } from '../lib/session.svelte.js';
  import Dialog from '../components/Dialog.svelte';
  import Icon from '../components/Icon.svelte';

  let seats = $state([]);
  let limit = $state(0);
  let active = $state(0);
  let error = $state('');
  let notice = $state(null); // kode kasir yang baru dibuat, ditampilkan sekali

  let addOpen = $state(false);
  let addName = $state('');
  let addError = $state('');
  let saving = $state(false);

  let renameOpen = $state(false);
  let renameTarget = $state(null);
  let renameName = $state('');
  let renameError = $state('');

  let codeOpen = $state(false);
  let codeTarget = $state(null);
  let codeError = $state('');

  let offOpen = $state(false);
  let offTarget = $state(null);
  let offError = $state('');

  const full = $derived(active >= limit);

  async function load() {
    try {
      const data = await api.get('/cashiers');
      seats = data.seats;
      limit = data.limit;
      active = data.active;
      error = '';
    } catch (e) {
      error = e.message;
    }
  }
  onMount(load);

  function openAdd() {
    addName = `Kasir ${seats.length + 1}`;
    addError = '';
    addOpen = true;
  }

  async function create() {
    saving = true;
    addError = '';
    try {
      const made = await api.post('/cashiers', { name: addName });
      notice = { title: 'Kasir dibuat', name: made.name, code: made.code };
      addOpen = false;
      await load();
    } catch (e) {
      addError = e.message;
    } finally {
      saving = false;
    }
  }

  function openRename(seat) {
    renameTarget = seat;
    renameName = seat.name;
    renameError = '';
    renameOpen = true;
  }

  async function rename() {
    try {
      await api.put(`/cashiers/${renameTarget.id}`, { name: renameName, is_active: renameTarget.is_active });
      renameOpen = false;
      await load();
    } catch (e) {
      renameError = e.message;
    }
  }

  function askNewCode(seat) {
    codeTarget = seat;
    codeError = '';
    codeOpen = true;
  }

  async function newCode() {
    try {
      const made = await api.post(`/cashiers/${codeTarget.id}/code`);
      codeOpen = false;
      notice = { title: 'Kode baru dibuat', name: made.name, code: made.code };
      await load();
    } catch (e) {
      codeError = e.message;
    }
  }

  async function setActive(seat, on) {
    try {
      await api.put(`/cashiers/${seat.id}`, { name: seat.name, is_active: on });
      offOpen = false;
      await load();
    } catch (e) {
      if (offOpen) offError = e.message;
      else error = e.message;
    }
  }

  function askDeactivate(seat) {
    offTarget = seat;
    offError = '';
    offOpen = true;
  }
</script>

<div class="mx-auto max-w-4xl space-y-5 p-4 lg:p-6">
  <div class="flex flex-wrap items-center gap-3">
    <div class="mr-auto min-w-0 basis-64">
      <h1 class="text-2xl font-bold">Akun kasir</h1>
      <p class="text-muted">Kasir hanya melihat layar kasir: tanpa stok, laporan, dan pengaturan. Semua penjualannya tetap tercatat di tokomu.</p>
    </div>
    <button type="button" class="btn btn-primary w-full sm:w-auto" onclick={openAdd} disabled={full}><Icon name="plus" /> Tambah kasir</button>
  </div>

  <p class="tabular-nums">Kasir aktif: <span class="font-semibold">{active} dari {limit}</span></p>
  {#if full}
    <p class="rounded-xl bg-tint px-4 py-3 text-muted">Jatah kasirmu sudah penuh. Nonaktifkan salah satu kasir, atau hubungi kami kalau butuh lebih dari {limit}.</p>
  {/if}

  {#if notice}
    <div role="status" class="space-y-3 rounded-2xl border border-emerald-300 bg-tint p-5 dark:border-emerald-800">
      <p class="font-semibold">{notice.title}: {notice.name}</p>
      <dl class="grid gap-1 sm:grid-cols-[auto_1fr] sm:gap-x-4">
        <dt class="text-muted">Username toko</dt>
        <dd class="select-all font-mono font-semibold">{session.user.username}</dd>
        <dt class="text-muted">Kode kasir</dt>
        <dd class="select-all break-all font-mono text-lg font-semibold tracking-widest sm:text-xl">{notice.code}</dd>
      </dl>
      <p class="text-sm text-muted">Catat dan berikan ke kasirnya sekarang. Kode ini tidak akan ditampilkan lagi. Kalau hilang atau kasirnya berhenti, buat kode baru: kode lama langsung tidak berlaku. Kasir masuk lewat halaman Masuk, pilih Kasir.</p>
      <button type="button" class="btn btn-quiet min-h-11 text-base" onclick={() => (notice = null)}>Sudah dicatat</button>
    </div>
  {/if}

  {#if error}
    <p role="alert" class="rounded-xl bg-red-50 p-4 text-red-800 dark:bg-red-950 dark:text-red-200">{error}</p>
  {/if}

  <ul class="space-y-3">
    {#each seats as seat (seat.id)}
      <li class="rounded-2xl border border-line bg-panel p-4 {seat.is_active ? '' : 'text-muted'}">
        <div class="flex items-center gap-3">
          <p class="min-w-0 flex-1 truncate text-lg font-semibold">{seat.name}</p>
          <span class="shrink-0 rounded-full px-3 py-1 text-sm font-semibold {seat.is_active ? 'bg-emerald-100 text-emerald-900 dark:bg-emerald-950 dark:text-emerald-200' : 'bg-line text-muted'}">{seat.is_active ? 'Aktif' : 'Nonaktif'}</span>
        </div>
        <div class="mt-3 grid grid-cols-2 gap-2 sm:flex sm:flex-wrap sm:justify-end">
          <button type="button" class="btn btn-quiet min-h-11 px-3 text-base" onclick={() => openRename(seat)}>Ubah nama</button>
          <button type="button" class="btn btn-quiet min-h-11 px-3 text-base" onclick={() => askNewCode(seat)}>Kode baru</button>
          {#if seat.is_active}
            <button type="button" class="btn btn-quiet col-span-2 min-h-11 px-3 text-base sm:col-span-1" onclick={() => askDeactivate(seat)}>Nonaktifkan</button>
          {:else}
            <button type="button" class="btn btn-quiet col-span-2 min-h-11 px-3 text-base sm:col-span-1" onclick={() => setActive(seat, true)}>Aktifkan</button>
          {/if}
        </div>
      </li>
    {:else}
      <li class="rounded-2xl border border-dashed border-line px-4 py-8 text-center text-muted">Belum ada kasir. Tambahkan yang pertama supaya karyawan bisa jualan tanpa melihat stok dan laporan.</li>
    {/each}
  </ul>
</div>

<!-- Tambah kasir -->
<Dialog bind:open={addOpen} label="Tambah kasir">
  <form
    class="space-y-4 p-6"
    onsubmit={(e) => {
      e.preventDefault();
      create();
    }}
  >
    <div>
      <h2 class="text-xl font-semibold">Tambah kasir</h2>
      <p class="text-muted">Kasir masuk dengan username tokomu dan kode khusus yang dibuat setelah ini. Kata sandimu tidak pernah dipakai kasir.</p>
    </div>
    <label class="block"><span class="mb-1 block font-medium">Nama kasir</span><input class="input" bind:value={addName} required maxlength="40" placeholder="Misalnya Kasir depan" /></label>
    {#if addError}<p role="alert" class="rounded-xl bg-red-50 px-4 py-3 text-red-800 dark:bg-red-950 dark:text-red-200">{addError}</p>{/if}
    <div class="flex gap-3">
      <button type="button" class="btn btn-quiet flex-1" onclick={() => (addOpen = false)}>Batal</button>
      <button type="submit" class="btn btn-primary flex-[2]" disabled={saving}>{saving ? 'Menyimpan…' : 'Buat kasir'}</button>
    </div>
  </form>
</Dialog>

<!-- Ubah nama -->
<Dialog bind:open={renameOpen} label="Ubah nama kasir">
  <form
    class="space-y-4 p-6"
    onsubmit={(e) => {
      e.preventDefault();
      rename();
    }}
  >
    <div>
      <h2 class="text-xl font-semibold">Ubah nama kasir</h2>
      <p class="text-muted">Struk lama tetap memakai nama saat transaksi terjadi.</p>
    </div>
    <input class="input" bind:value={renameName} required maxlength="40" aria-label="Nama kasir" />
    {#if renameError}<p role="alert" class="rounded-xl bg-red-50 px-4 py-3 text-red-800 dark:bg-red-950 dark:text-red-200">{renameError}</p>{/if}
    <div class="flex gap-3">
      <button type="button" class="btn btn-quiet flex-1" onclick={() => (renameOpen = false)}>Batal</button>
      <button type="submit" class="btn btn-primary flex-[2]">Simpan</button>
    </div>
  </form>
</Dialog>

<!-- Konfirmasi kode baru -->
<Dialog bind:open={codeOpen} label="Buat kode baru">
  <div class="space-y-4 p-6">
    <div>
      <h2 class="text-xl font-semibold">Buat kode baru untuk {codeTarget?.name}?</h2>
      <p class="text-muted">Kode lama langsung tidak berlaku dan kasir ini keluar dari semua perangkat. Pakai ini kalau kodenya hilang, bocor, atau kasirnya berhenti.</p>
    </div>
    {#if codeError}<p role="alert" class="rounded-xl bg-red-50 px-4 py-3 text-red-800 dark:bg-red-950 dark:text-red-200">{codeError}</p>{/if}
    <div class="flex gap-3">
      <button type="button" class="btn btn-quiet flex-1" onclick={() => (codeOpen = false)}>Batal</button>
      <button type="button" class="btn btn-primary flex-[2]" onclick={newCode}>Buat kode baru</button>
    </div>
  </div>
</Dialog>

<!-- Konfirmasi nonaktifkan -->
<Dialog bind:open={offOpen} label="Nonaktifkan kasir">
  <div class="space-y-4 p-6">
    <div>
      <h2 class="text-xl font-semibold">Nonaktifkan {offTarget?.name}?</h2>
      <p class="text-muted">Kasir ini langsung keluar dari semua perangkat dan tidak bisa masuk lagi. Riwayat penjualannya tetap tersimpan, dan jatahnya bisa dipakai kasir lain.</p>
    </div>
    {#if offError}<p role="alert" class="rounded-xl bg-red-50 px-4 py-3 text-red-800 dark:bg-red-950 dark:text-red-200">{offError}</p>{/if}
    <div class="flex gap-3">
      <button type="button" class="btn btn-quiet flex-1" onclick={() => (offOpen = false)}>Batal</button>
      <button type="button" class="btn btn-primary flex-[2]" onclick={() => setActive(offTarget, false)}>Nonaktifkan</button>
    </div>
  </div>
</Dialog>
