<script>
  import { onMount } from 'svelte';
  import { api } from '../lib/api.js';
  import { money, moneyCompact } from '../lib/money.js';
  import { navigate } from '../lib/router.svelte.js';
  import Icon from '../components/Icon.svelte';

  const ranges = [7, 14, 30];
  const todayLabel = new Date().toLocaleDateString(undefined, { weekday: 'long', day: 'numeric', month: 'long' });

  let days = $state(7);
  let data = $state(null);
  let low = $state([]);
  let error = $state('');
  let refreshing = $state(false);
  let updatedAt = $state(null);
  let latest = 0;

  async function load(range) {
    const mine = ++latest; // ignore answers that arrive after a newer request
    refreshing = true;
    try {
      const [summary, lowStock] = await Promise.all([
        api.get(`/reports/summary?days=${range}`),
        api.get('/products?low_stock=true&limit=9'),
      ]);
      if (mine !== latest) return;
      data = summary;
      low = lowStock;
      updatedAt = new Date();
      error = '';
    } catch (e) {
      if (mine === latest) error = e.message;
    } finally {
      if (mine === latest) refreshing = false;
    }
  }

  // Loads on open, when the range changes, and quietly once a minute while the page is open.
  $effect(() => {
    const range = days;
    load(range);
    const timer = setInterval(() => load(range), 60_000);
    return () => clearInterval(timer);
  });

  const daily = $derived(data?.daily ?? []);
  const today = $derived(data?.today ?? { revenue_cents: 0, orders: 0 });
  const earlier = $derived(daily.slice(0, -1));
  const usual = $derived(earlier.length ? Math.round(earlier.reduce((s, d) => s + d.revenue_cents, 0) / earlier.length) : 0);
  const pct = $derived(usual > 0 ? Math.round((today.revenue_cents / usual) * 100) : null);
  const rangeRevenue = $derived(daily.reduce((s, d) => s + d.revenue_cents, 0));
  const rangeOrders = $derived(daily.reduce((s, d) => s + d.orders, 0));
  const avgSale = $derived(rangeOrders ? Math.round(rangeRevenue / rangeOrders) : 0);
  const highest = $derived(Math.max(0, ...daily.map((d) => d.revenue_cents)));
  const scale = $derived(Math.max(1, highest));
  const topMax = $derived(Math.max(1, ...(data?.top_items ?? []).map((t) => t.quantity)));

  const parse = (iso) => new Date(`${iso}T00:00:00`);
  const longDay = (iso) => parse(iso).toLocaleDateString(undefined, { weekday: 'long', day: 'numeric', month: 'long' });
  const shortDay = (iso) => parse(iso).toLocaleDateString(undefined, { weekday: 'short' });
  // Seven bars: weekday names. More bars: day-of-month, thinned out so the labels don't collide.
  const dayLabel = (iso, i, n) => {
    if (n <= 7) return shortDay(iso);
    return i % (n <= 14 ? 2 : 5) === 0 || i === n - 1 ? String(parse(iso).getDate()) : '';
  };
  const salesWord = (n) => (n === 1 ? 'sale' : 'sales');
</script>

<div class="mx-auto max-w-5xl space-y-8 p-4 lg:p-6">
  <!-- Top bar: today's date on the left, range and refresh on the right -->
  <div class="flex flex-wrap items-center justify-between gap-3">
    <p class="font-medium text-muted">{todayLabel}</p>
    <div class="flex flex-wrap items-center gap-3">
      <div class="flex gap-2" role="group" aria-label="Date range">
        {#each ranges as r (r)}
          <button type="button" class="chip" aria-pressed={days === r} onclick={() => (days = r)}>{r} days</button>
        {/each}
      </div>
      <button type="button" class="chip gap-2" onclick={() => load(days)} disabled={refreshing} aria-label="Refresh numbers">
        <Icon name="refresh" class="size-4 {refreshing ? 'animate-spin motion-reduce:animate-none' : ''}" />
        {#if updatedAt}<span class="tabular-nums">{updatedAt.toLocaleTimeString(undefined, { hour: 'numeric', minute: '2-digit' })}</span>{:else}Refresh{/if}
      </button>
    </div>
  </div>

  {#if error && !data}
    <div role="alert" class="flex flex-wrap items-center gap-4 rounded-2xl bg-red-50 p-5 text-red-800 dark:bg-red-950 dark:text-red-200">
      <Icon name="alert" class="size-5 shrink-0" />
      <p class="flex-1">{error}</p>
      <button type="button" class="btn btn-quiet min-h-11 text-base" onclick={() => load(days)}>Try again</button>
    </div>
  {:else if !data}
    <div class="animate-pulse space-y-6 motion-reduce:animate-none" aria-hidden="true">
      <div class="h-12 max-w-2xl rounded-xl bg-line"></div>
      <div class="h-2 max-w-md rounded-full bg-line"></div>
      <div class="h-80 rounded-2xl bg-line/60"></div>
    </div>
    <p class="sr-only" role="status">Loading today’s numbers</p>
  {:else}
    <!-- Today, as a plain sentence -->
    <section aria-label="Today" class="space-y-4">
      <h1 class="max-w-3xl text-3xl font-semibold leading-tight tracking-tight tabular-nums sm:text-4xl">
        {#if today.orders > 0}
          You’ve taken {money(today.revenue_cents)} from {today.orders} {salesWord(today.orders)} today.
        {:else}
          No sales yet today.
        {/if}
      </h1>

      {#if pct !== null}
        <div class="max-w-md">
          <div class="h-2 overflow-hidden rounded-full bg-line">
            <div class="h-full rounded-full bg-emerald-600" style:width="{Math.min(100, pct)}%"></div>
          </div>
          <p class="mt-2 text-muted">
            {pct >= 100 ? 'Already past a usual day.' : `${pct}% of a usual day so far.`}
            A usual day brings in {money(usual)}.
          </p>
        </div>
      {:else if rangeOrders === 0}
        <p class="max-w-md text-muted">Your first sale will show up here as soon as you ring it up.</p>
      {/if}
    </section>

    <!-- Sales by day -->
    <section class="rounded-2xl border border-line bg-panel p-5 sm:p-6" aria-label="Sales by day" aria-busy={refreshing}>
      <header class="flex flex-wrap items-end justify-between gap-x-8 gap-y-3">
        <h2 class="text-lg font-semibold">Sales, last {days} days</h2>
        <dl class="flex gap-6">
          <div><dt class="text-sm text-muted">Total</dt><dd class="text-lg font-semibold tabular-nums">{money(rangeRevenue)}</dd></div>
          <div><dt class="text-sm text-muted">Sales</dt><dd class="text-lg font-semibold tabular-nums">{rangeOrders}</dd></div>
          <div><dt class="text-sm text-muted">Average sale</dt><dd class="text-lg font-semibold tabular-nums">{money(avgSale)}</dd></div>
        </dl>
      </header>

      <!-- Bar heights and positions use style: directives. Those set styles from script, so they work under the strict CSP. -->
      <div class="mt-10" role="img" aria-label="Bar chart of sales for each of the last {days} days. The table below has the same numbers.">
        <div class="relative h-44 border-b border-line">
          {#if usual > 0}
            <div class="absolute inset-x-0 border-t border-dashed border-emerald-600/70" style:bottom="{(usual / scale) * 100}%">
              <span class="absolute -top-6 right-0 bg-panel pl-2 text-xs text-muted">Usual day</span>
            </div>
          {/if}
          <ol class="absolute inset-0 flex gap-1.5 sm:gap-2.5" aria-hidden="true">
            {#each daily as d, i (d.date)}
              {@const isToday = i === daily.length - 1}
              {@const h = d.revenue_cents > 0 ? Math.max(2, (d.revenue_cents / scale) * 100) : 0}
              <li class="relative flex h-full min-w-0 flex-1 items-end" title="{longDay(d.date)}: {money(d.revenue_cents)} from {d.orders} {salesWord(d.orders)}">
                <div
                  class="bar-rise min-h-[3px] w-full rounded-t-md {isToday ? 'bg-emerald-600' : 'bg-sage-300 dark:bg-emerald-800'}"
                  style:height="{h}%"
                  style:animation-delay="{i * 25}ms"
                ></div>
                {#if d.revenue_cents > 0 && (isToday || d.revenue_cents === highest)}
                  <span class="absolute inset-x-[-1rem] mb-1 text-center text-xs font-semibold tabular-nums" style:bottom="{h}%">{moneyCompact(d.revenue_cents)}</span>
                {/if}
              </li>
            {/each}
          </ol>
        </div>
        <ol class="mt-2 flex gap-1.5 sm:gap-2.5" aria-hidden="true">
          {#each daily as d, i (d.date)}
            <li class="min-w-0 flex-1 text-center text-xs {i === daily.length - 1 ? 'font-semibold text-ink' : 'text-muted'}">{dayLabel(d.date, i, daily.length)}</li>
          {/each}
        </ol>
      </div>

      <table class="sr-only">
        <caption>Sales for each day</caption>
        <thead><tr><th>Day</th><th>Sales</th><th>Number of sales</th></tr></thead>
        <tbody>
          {#each daily as d (d.date)}
            <tr><td>{longDay(d.date)}</td><td>{money(d.revenue_cents)}</td><td>{d.orders}</td></tr>
          {/each}
        </tbody>
      </table>
    </section>

    <div class="grid gap-8 lg:grid-cols-2">
      <!-- Fast-moving -->
      <section class="rounded-2xl border border-line bg-panel p-5 sm:p-6" aria-label="Fast-moving items">
        <h2 class="mb-4 text-lg font-semibold">Fast-moving items</h2>
        {#if data.top_items.length === 0}
          <p class="text-muted">Nothing has sold in the last {days} days. Your best sellers will rank here.</p>
        {:else}
          <ol class="space-y-4">
            {#each data.top_items as item, i (item.product_id)}
              <li>
                <div class="flex items-baseline justify-between gap-3">
                  <span class="flex min-w-0 items-baseline gap-3">
                    <span class="w-4 shrink-0 text-sm tabular-nums text-muted">{i + 1}</span>
                    <span class="truncate font-semibold">{item.name}</span>
                  </span>
                  <span class="shrink-0 tabular-nums text-muted">{item.quantity} sold</span>
                </div>
                <div class="ml-7 mt-2 h-2 rounded-full bg-line">
                  <div class="bar-grow h-full rounded-full bg-emerald-600" style:width="{(item.quantity / topMax) * 100}%" style:animation-delay="{i * 60}ms"></div>
                </div>
                <p class="ml-7 mt-1 text-sm tabular-nums text-muted">{money(item.revenue_cents)} in sales</p>
              </li>
            {/each}
          </ol>
        {/if}
      </section>

      <!-- Running low -->
      <section class="flex flex-col rounded-2xl border border-line bg-panel p-5 sm:p-6" aria-label="Running low">
        <h2 class="mb-2 text-lg font-semibold">Running low</h2>
        {#if low.length === 0}
          <div class="flex items-center gap-3 py-4 text-muted">
            <span class="grid size-9 shrink-0 place-items-center rounded-full bg-tint text-emerald-700 dark:text-emerald-400"><Icon name="check" class="size-5" /></span>
            Nothing is running low. Stock levels look healthy.
          </div>
        {:else}
          <ul class="divide-y divide-line">
            {#each low.slice(0, 8) as p (p.id)}
              <li class="flex items-center justify-between gap-3 py-3">
                <span class="min-w-0">
                  <span class="block truncate font-semibold">{p.name}</span>
                  <span class="text-sm text-muted">Warns at {p.low_stock_threshold} or fewer</span>
                </span>
                <span class="shrink-0 rounded-full px-3 py-1 text-sm font-semibold tabular-nums {p.stock_qty === 0 ? 'bg-red-100 text-red-800 dark:bg-red-950 dark:text-red-200' : 'bg-amber-100 text-amber-900 dark:bg-amber-950 dark:text-amber-200'}">
                  {p.stock_qty === 0 ? 'Out of stock' : `${p.stock_qty} left`}
                </span>
              </li>
            {/each}
          </ul>
          {#if low.length > 8}<p class="pt-2 text-sm text-muted">More items are running low. Open Stock to see them all.</p>{/if}
          <div class="mt-auto pt-4">
            <button type="button" class="btn btn-quiet min-h-11 text-base" onclick={() => navigate('/inventory')}>Open stock</button>
          </div>
        {/if}
      </section>
    </div>
  {/if}
</div>
