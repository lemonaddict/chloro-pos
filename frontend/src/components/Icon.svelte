<script>
  // Small inline icon set (24px, stroke). No icon package to install, and nothing inline-styled,
  // so it works under the strict Content-Security-Policy.
  let { name, class: className = 'size-5' } = $props();

  const paths = {
    user: ['M19 21v-2a4 4 0 0 0-4-4H9a4 4 0 0 0-4 4v2', 'M12 3a4 4 0 1 0 0 8 4 4 0 0 0 0-8Z'],
    lock: ['M5 11h14a2 2 0 0 1 2 2v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-7a2 2 0 0 1 2-2Z', 'M7 11V7a5 5 0 0 1 10 0v4'],
    eye: ['M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12Z', 'M12 9a3 3 0 1 0 0 6 3 3 0 0 0 0-6Z'],
    eyeOff: ['M2 12s3.5-7 10-7c2 0 3.7.6 5.2 1.5', 'M22 12s-3.5 7-10 7c-2 0-3.7-.6-5.2-1.5', 'M3 3l18 18'],
    bolt: ['M13 2 3 14h9l-1 8 10-12h-9l1-8Z'],
    stock: ['M4 8l8-4 8 4v8l-8 4-8-4V8Z', 'M4 8l8 4 8-4', 'M12 12v8'],
    chart: ['M5 20V10', 'M12 20V4', 'M19 20v-7'],
    alert: ['M12 9v4', 'M12 17h.01', 'M10.3 3.9 1.8 18a2 2 0 0 0 1.7 3h17a2 2 0 0 0 1.7-3L13.7 3.9a2 2 0 0 0-3.4 0Z'],
    refresh: ['M21 12a9 9 0 1 1-3-6.7L21 8', 'M21 3v5h-5'],
    check: ['M20 6 9 17l-5-5'],
  };
</script>

<svg viewBox="0 0 24 24" class={className} fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
  {#each paths[name] ?? [] as d}
    <path {d} />
  {/each}
</svg>
