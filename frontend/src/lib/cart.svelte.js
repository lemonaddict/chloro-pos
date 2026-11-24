// The cart holds display copies only. The server re-prices every line (and adds tax) at checkout.
class Cart {
  lines = $state([]);
  total = $derived(this.lines.reduce((sum, l) => sum + l.qty * l.price_cents, 0));

  /** Returns false when the product can't be added (out of stock, or all known stock is already in the cart). */
  add(product) {
    if (product.stock_status === 'out') return false;
    // Cashiers don't receive stock counts, so there is no cap here. The server enforces stock at checkout.
    const limit = product.stock_qty ?? Infinity;
    const line = this.lines.find((l) => l.id === product.id);
    const have = line ? line.qty : 0;
    if (have >= limit) return false;
    if (line) line.qty = have + 1;
    else this.lines.push({ id: product.id, name: product.name, price_cents: product.price_cents, limit, qty: 1 });
    return true;
  }

  change(id, delta) {
    const line = this.lines.find((l) => l.id === id);
    if (!line) return;
    const next = line.qty + delta;
    if (next <= 0) this.remove(id);
    else if (next <= line.limit) line.qty = next;
  }

  remove(id) {
    this.lines = this.lines.filter((l) => l.id !== id);
  }

  clear() {
    this.lines = [];
  }
}

export const cart = new Cart();
