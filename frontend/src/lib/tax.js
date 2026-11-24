// Mirrors ComputeTax in the Go backend. The server is the one that counts; this only previews it.
//   Exclusive: prices exclude tax, total = subtotal + tax.
//   Inclusive: prices already include tax, total = subtotal, tax is the part inside it.
export function computeTax(subtotal, { tax_enabled, tax_rate_bps, tax_inclusive }) {
  if (!tax_enabled || tax_rate_bps <= 0) return { tax: 0, total: subtotal };
  const rate = tax_rate_bps;
  const rupiah = Math.floor(subtotal / 100);
  if (tax_inclusive) {
    const den = 10000 + rate;
    return { tax: Math.floor((rupiah * rate + Math.floor(den / 2)) / den) * 100, total: subtotal };
  }
  const tax = Math.floor((rupiah * rate + 5000) / 10000) * 100;
  return { tax, total: subtotal + tax };
}
