<script>
  // A single leaf on a green tile. No animation of its own; pages decide whether to animate it.
  let { size = 'size-10', class: className = '' } = $props();
</script>

<svg viewBox="0 0 32 32" class="{size} {className}" role="img" aria-label="ChloroPOS">
  <rect width="32" height="32" rx="9" fill="#059669" />
  <path d="M9 23C9 14 14 8 24 8c0 10-6 15-15 15Z" fill="#ecfdf5" />
  <path d="M9 23 19 13" stroke="#059669" stroke-width="1.8" stroke-linecap="round" fill="none" />
</svg>
