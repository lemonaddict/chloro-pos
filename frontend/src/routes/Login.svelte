<script>
  import { onMount } from 'svelte';
  import { signIn } from '../lib/session.svelte.js';
  import { sampleMoney } from '../lib/money.js';
  import Icon from '../components/Icon.svelte';

  const shop = import.meta.env.VITE_SHOP_NAME || 'Your Shop';
  const day = new Date().toLocaleDateString(undefined, { day: 'numeric', month: 'short' });

  // A made-up sale for the receipt on the left. The numbers add up on purpose.
  const sale = [
    { qty: 2, name: 'Flat white', price: 7 },
    { qty: 1, name: 'Almond croissant', price: 3.5 },
    { qty: 1, name: 'Beans 250 g', price: 12 },
  ];

  const perks = [
    { icon: 'bolt', text: 'Charge in one tap, or press F1' },
    { icon: 'stock', text: 'Stock updates with every sale' },
    { icon: 'chart', text: 'Today’s totals ready when you close' },
  ];

  let username = $state('');
  let password = $state('');
  let show = $state(false);
  let capsOn = $state(false);
  let busy = $state(false);
  let error = $state('');
  let userEl;

  onMount(() => userEl?.focus());

  const checkCaps = (e) => (capsOn = e.getModifierState?.('CapsLock') ?? false);

  async function submit(event) {
    event.preventDefault();
    busy = true;
    error = '';
    try {
      await signIn(username.trim(), password);
    } catch (e) {
      error = e.message;
      password = '';
    } finally {
      busy = false;
    }
  }
</script>

<main class="grid min-h-dvh bg-page lg:grid-cols-[5fr_6fr]">
  <!-- Phones and small tablets: a slim brand bar instead of the full panel -->
  <header class="flex items-center gap-3 bg-forest px-6 py-5 text-white lg:hidden">
    <div class="grid size-10 place-items-center rounded-xl bg-emerald-600 text-lg font-bold" aria-hidden="true">P</div>
    <span class="text-lg font-semibold">{shop}</span>
  </header>

  <aside class="hidden flex-col justify-between gap-10 overflow-hidden bg-forest p-12 text-white lg:flex xl:p-16">
    <div class="flex items-center gap-3">
      <div class="grid size-11 place-items-center rounded-xl bg-emerald-600 text-xl font-bold" aria-hidden="true">P</div>
      <span class="text-xl font-semibold">{shop}</span>
    </div>

    <!-- The one memorable thing on this page: the receipt prints itself once on load -->
    <div class="flex justify-center py-2" aria-hidden="true">
      <div class="slip-print w-72 -rotate-2 select-none drop-shadow-2xl">
        <div class="bg-paper px-6 pb-4 pt-6 font-mono text-[13px] leading-6 text-forest">
          <p class="text-center text-base font-bold">{shop}</p>
          <p class="text-center text-sage-500">Receipt 000128, {day}</p>
          <ul class="my-3 space-y-0.5 border-y border-dashed border-sage-300 py-3">
            {#each sale as line}
              <li class="flex justify-between gap-3">
                <span>{line.qty} × {line.name}</span>
                <span class="tabular-nums">{sampleMoney(line.price)}</span>
              </li>
            {/each}
          </ul>
          <p class="flex justify-between font-bold"><span>Total</span><span class="tabular-nums">{sampleMoney(22.5)}</span></p>
          <p class="flex justify-between"><span>Cash</span><span class="tabular-nums">{sampleMoney(25)}</span></p>
          <p class="flex justify-between"><span>Change</span><span class="tabular-nums">{sampleMoney(2.5)}</span></p>
          <p class="mt-3 text-center">Thank you!</p>
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
      <h2 class="max-w-sm text-3xl font-semibold leading-tight tracking-tight">Sell fast. Keep stock honest.</h2>
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
        <h1 class="text-3xl font-semibold tracking-tight">Sign in</h1>
        <p class="text-muted">Open the till with the username your shop owner gave you.</p>
      </div>

      <div>
        <label for="username" class="mb-1.5 block font-medium">Username</label>
        <div class="relative">
          <Icon name="user" class="pointer-events-none absolute left-4 top-1/2 size-5 -translate-y-1/2 text-muted" />
          <input
            id="username"
            bind:this={userEl}
            bind:value={username}
            class="input pl-12"
            autocomplete="username"
            autocapitalize="none"
            spellcheck="false"
            required
          />
        </div>
      </div>

      <div>
        <label for="password" class="mb-1.5 block font-medium">Password</label>
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
            aria-label={show ? 'Hide password' : 'Show password'}
            aria-pressed={show}
            onclick={() => (show = !show)}
          >
            <Icon name={show ? 'eyeOff' : 'eye'} />
          </button>
        </div>
        {#if capsOn}
          <p class="mt-2 flex items-center gap-2 text-sm font-medium text-amber-800 dark:text-amber-300">
            <Icon name="alert" class="size-4 shrink-0" /> Caps Lock is on.
          </p>
        {/if}
      </div>

      {#if error}
        <div role="alert" class="flex gap-3 rounded-xl bg-red-50 px-4 py-3 text-red-800 dark:bg-red-950 dark:text-red-200">
          <Icon name="alert" class="mt-0.5 size-5 shrink-0" />
          <p>{error}</p>
        </div>
      {/if}

      <button type="submit" class="btn btn-primary h-14 w-full text-xl" disabled={busy}>
        {busy ? 'Signing in…' : 'Sign in'}
      </button>

      <p class="text-sm text-muted">No account yet? Ask the shop owner to add you.</p>
    </form>
  </section>
</main>
