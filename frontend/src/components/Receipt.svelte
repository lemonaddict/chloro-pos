<script>
  import { money } from '../lib/money.js';

  let { order, onnew } = $props();

  const shop = import.meta.env.VITE_SHOP_NAME || 'Your Shop';
  const when = $derived(new Date(order.created_at).toLocaleString());
  const paidWith = $derived(order.payment_method[0].toUpperCase() + order.payment_method.slice(1));
</script>

<div class="p-6">
  <div id="receipt" class="space-y-3 font-mono text-sm">
    <h2 class="text-center text-lg font-bold">{shop}</h2>
    <p class="text-center text-muted">Receipt {String(order.receipt_no).padStart(6, '0')} on {when}</p>

    <ul class="divide-y divide-dashed divide-line border-y border-dashed border-line">
      {#each order.items as item (item.product_id)}
        <li class="flex justify-between gap-3 py-2">
          <span>{item.quantity} × {item.product_name}</span>
          <span class="tabular-nums">{money(item.line_total_cents)}</span>
        </li>
      {/each}
    </ul>

    <dl class="space-y-1">
      <div class="flex justify-between text-base font-bold"><dt>Total</dt><dd class="tabular-nums">{money(order.total_cents)}</dd></div>
      <div class="flex justify-between"><dt>{paidWith}</dt><dd class="tabular-nums">{money(order.amount_tendered_cents)}</dd></div>
      {#if order.change_cents > 0}
        <div class="flex justify-between"><dt>Change</dt><dd class="tabular-nums">{money(order.change_cents)}</dd></div>
      {/if}
    </dl>
    <p class="pt-2 text-center">Thank you!</p>
  </div>

  <div class="mt-5 flex gap-3 print:hidden">
    <button type="button" class="btn btn-quiet flex-1" onclick={() => window.print()}>Print</button>
    <button type="button" class="btn btn-primary flex-[2]" onclick={onnew}>New sale</button>
  </div>
</div>
