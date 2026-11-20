// Amounts travel as integer minor units (cents). Convert only at the screen edge.
const currency = import.meta.env.VITE_CURRENCY || 'USD';
const formatter = new Intl.NumberFormat(undefined, { style: 'currency', currency });

export const money = (cents) => formatter.format(cents / 100);

// "12.5" or "12,50" -> 1250. Anything unreadable becomes 0.
export function toCents(text) {
  const n = Number.parseFloat(String(text).replace(',', '.'));
  return Number.isFinite(n) && n >= 0 ? Math.round(n * 100) : 0;
}

// Up to three round amounts above the total that a customer is likely to hand over.
export function cashSuggestions(totalCents) {
  const steps = [1, 5, 10, 20, 50, 100, 500, 1000, 5000, 10000, 50000, 100000].map((s) => s * 100);
  const out = new Set();
  for (const step of steps) {
    const rounded = Math.ceil(totalCents / step) * step;
    if (rounded > totalCents) out.add(rounded);
    if (out.size === 3) break;
  }
  return [...out];
}

// Short form for chart labels: $1.2K instead of $1,234.00.
const compact = new Intl.NumberFormat(undefined, { style: 'currency', currency, notation: 'compact', maximumFractionDigits: 1 });
export const moneyCompact = (cents) => compact.format(cents / 100);

// The sign-in page shows a sample receipt. Scale its prices so a coffee doesn't cost "Rp 3,50".
const SAMPLE_SCALE = { IDR: 8000, JPY: 140, KRW: 1300, VND: 24000, INR: 80, PHP: 55, THB: 35 };
export const sampleMoney = (major) => money(Math.round(major * (SAMPLE_SCALE[currency.toUpperCase()] ?? 1) * 100));
