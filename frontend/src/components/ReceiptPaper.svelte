<script>
  // The receipt itself. Used for the real printout (printable) and for the live preview in Pengaturan.
  // Always dark ink on light paper — thermal paper look must stay readable in dark mode of the app.
  import { money } from '../lib/money.js';
  import { formatNpwp, dateTime, percent } from '../lib/format.js';

  let { order, biz, printable = true } = $props();

  const methods = { cash: 'Tunai', qris: 'QRIS', transfer: 'Transfer', card: 'Kartu' };
  const taxed = $derived(order.tax_cents > 0 && order.tax_label);
  const rate = $derived(percent(order.tax_rate_bps));
</script>

<div id={printable ? 'receipt' : undefined} class="space-y-2 font-mono text-[13px] leading-5 text-forest">
  <div class="text-center">
    <p class="text-base font-bold">{biz.business_name}</p>
    {#if biz.legal_name}<p>{biz.legal_name}</p>{/if}
    {#if biz.address}<p class="whitespace-pre-line">{biz.address}</p>{/if}
    {#if biz.phone}<p>Telp. {biz.phone}</p>{/if}
    {#if biz.email}<p>{biz.email}</p>{/if}
    {#if biz.npwp}<p>NPWP {formatNpwp(biz.npwp)}</p>{/if}
  </div>

  <div class="border-t border-dashed border-sage-300"></div>
  <div>
    <p class="flex justify-between gap-3"><span>No. {String(order.receipt_no).padStart(6, '0')}</span><span>{dateTime(order.created_at)}</span></p>
    <p>Kasir: {order.cashier_name}</p>
  </div>
  <div class="border-t border-dashed border-sage-300"></div>

  <ul class="space-y-1.5">
    {#each order.items as item (item.product_id)}
      <li>
        <p class="font-semibold">{item.product_name}</p>
        <p class="flex justify-between gap-3">
          <span>{item.quantity} x {money(item.unit_price_cents)}</span>
          <span class="tabular-nums">{money(item.line_total_cents)}</span>
        </p>
      </li>
    {/each}
  </ul>

  <div class="border-t border-dashed border-sage-300"></div>
  <dl class="space-y-0.5">
    {#if taxed && !order.tax_inclusive}
      <div class="flex justify-between gap-3"><dt>Subtotal</dt><dd class="tabular-nums">{money(order.subtotal_cents)}</dd></div>
      <div class="flex justify-between gap-3"><dt>{order.tax_label} {rate}%</dt><dd class="tabular-nums">{money(order.tax_cents)}</dd></div>
    {/if}
    <div class="flex justify-between gap-3 text-base font-bold"><dt>Total</dt><dd class="tabular-nums">{money(order.total_cents)}</dd></div>
    <div class="flex justify-between gap-3"><dt>{methods[order.payment_method]}</dt><dd class="tabular-nums">{money(order.amount_tendered_cents)}</dd></div>
    {#if order.change_cents > 0}
      <div class="flex justify-between gap-3"><dt>Kembali</dt><dd class="tabular-nums">{money(order.change_cents)}</dd></div>
    {/if}
  </dl>
  {#if taxed && order.tax_inclusive}
    <p class="text-center text-xs">Harga sudah termasuk {order.tax_label} {rate}%: {money(order.tax_cents)}</p>
  {/if}

  {#if biz.receipt_footer}
    <div class="border-t border-dashed border-sage-300"></div>
    <p class="whitespace-pre-line text-center">{biz.receipt_footer}</p>
  {/if}
</div>
