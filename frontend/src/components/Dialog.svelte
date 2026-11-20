<script>
  // A thin wrapper over the native <dialog>: focus trapping, Esc to close and a backdrop for free.
  let { open = $bindable(false), label, children } = $props();
  let el = $state();

  $effect(() => {
    if (!el) return;
    if (open && !el.open) el.showModal();
    else if (!open && el.open) el.close();
  });
</script>

<dialog
  bind:this={el}
  aria-label={label}
  onclose={() => (open = false)}
  onclick={(e) => { if (e.target === el) open = false; }}
  class="m-auto max-h-[calc(100dvh-2rem)] w-[min(32rem,calc(100vw-2rem))] overflow-y-auto rounded-3xl bg-panel p-0 text-ink shadow-2xl backdrop:bg-black/50 print:max-h-none print:shadow-none"
>
  {#if open}
    {@render children()}
  {/if}
</dialog>
