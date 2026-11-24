// Amounts travel as integers in minor units (x100) so the API never handles fractions.
// Rupiah has no sen in practice, so on screen everything is whole rupiah.
const idr = new Intl.NumberFormat('id-ID', { style: 'currency', currency: 'IDR', minimumFractionDigits: 0, maximumFractionDigits: 0 });
const compact = new Intl.NumberFormat('id-ID', { style: 'currency', currency: 'IDR', notation: 'compact', maximumFractionDigits: 1 });

export const money = (minor) => idr.format(minor / 100);
export const moneyCompact = (minor) => compact.format(minor / 100);

/** Minor units -> plain digits for an input box: 1500000 -> "15000". */
export const toInput = (minor) => String(Math.round(minor / 100));

/** What a person types ("15000", "15.000", "Rp 15.000,00") -> minor units. Anything unreadable is 0. */
export function toMinor(text) {
  const digits = String(text).replace(/,\d{1,2}$/, '').replace(/\D/g, '').slice(0, 12);
  return digits ? Number(digits) * 100 : 0;
}

/** Up to three round amounts above the total that a customer is likely to hand over. */
export function cashSuggestions(totalMinor) {
  const steps = [1000, 2000, 5000, 10000, 20000, 50000, 100000].map((rp) => rp * 100);
  const out = new Set();
  for (const step of steps) {
    const rounded = Math.ceil(totalMinor / step) * step;
    if (rounded > totalMinor) out.add(rounded);
    if (out.size === 3) break;
  }
  return [...out];
}
