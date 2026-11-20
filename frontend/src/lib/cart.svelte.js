// The cart holds display copies only. The server re-prices every line at checkout.
class Cart {
  lines = $state([]);
  total = $derived(this.lines.reduce((sum, l) => sum + l.qty * l.price_cents, 0));

  /** Returns false when there is no stock left to add. */
  add(product) {
    const line = this.lines.find((l) => l.id === product.id);
    const have = line ? line.qty : 0;
    if (have >= product.stock_qty) return false;
    if (line) line.qty = have + 1;
    else this.lines.push({ id: product.id, name: product.name, price_cents: product.price_cents, stock_qty: product.stock_qty, qty: 1 });
    return true;
  }

  change(id, delta) {
    const line = this.lines.find((l) => l.id === id);
    if (!line) return;
    const next = line.qty + delta;
    if (next <= 0) this.remove(id);
    else if (next <= line.stock_qty) line.qty = next;
  }

  remove(id) {
    this.lines = this.lines.filter((l) => l.id !== id);
  }

  clear() {
    this.lines = [];
  }
}

export const cart = new Cart();
