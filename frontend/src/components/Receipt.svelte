<script>
  import { settings } from '../lib/settings.svelte.js';
  import ReceiptPaper from './ReceiptPaper.svelte';
  import Icon from './Icon.svelte';

  let { order, onnew } = $props();
</script>

<div class="p-6">
  <div class="mx-auto max-w-sm rounded-2xl border border-line bg-paper p-5 shadow-sm">
    <ReceiptPaper {order} biz={settings} />
  </div>
  <div class="mt-5 flex gap-3 print:hidden">
    <button type="button" class="btn btn-quiet flex-1" onclick={() => window.print()}>
      <Icon name="printer" /> Cetak
    </button>
    <button type="button" class="btn btn-primary flex-[2]" onclick={onnew}>Penjualan baru</button>
  </div>
</div>
