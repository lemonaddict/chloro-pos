export function formatNpwp(digits) {
  if (digits.length === 15) {
    return `${digits.slice(0, 2)}.${digits.slice(2, 5)}.${digits.slice(5, 8)}.${digits.slice(8, 9)}-${digits.slice(9, 12)}.${digits.slice(12)}`;
  }
  if (digits.length === 16) return digits.replace(/(\d{4})(?=\d)/g, '$1 ');
  return digits;
}

export const dateTime = (iso) =>
  new Date(iso).toLocaleString('id-ID', { day: 'numeric', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit' });

/** 1100 -> "11", 1150 -> "11,5" */
export const percent = (bps) => (bps / 100).toLocaleString('id-ID', { maximumFractionDigits: 2 });

/** Local calendar date as YYYY-MM-DD (toISOString would shift it to UTC). */
export const ymd = (d) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
