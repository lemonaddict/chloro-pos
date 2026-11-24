package reporting

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/xuri/excelize/v2"

	"chloropos/internal/platform/tenancy"
)

// rowLimit caps each sheet. One more row is read so we can tell the person it was cut.
const rowLimit = 50_000

type ExportOrder struct {
	ReceiptNo                                        int64
	At                                               time.Time
	Cashier, Payment                                 string
	Net, Tax, Total, Tendered, Change                int64
}

type ExportLine struct {
	ReceiptNo        int64
	At               time.Time
	Product          string
	Qty              int
	Unit, LineTotal  int64
}

type ExportStock struct {
	SKU, Name, Category string
	Price               int64
	Qty, Threshold      int
	Active              bool
}

type ExportMove struct {
	At              time.Time
	Product         string
	Delta           int
	Reason, By      string
}

type ExportDay struct {
	Date                   time.Time
	Orders, Net, Tax, Gross int64
}

type ExportData struct {
	From, To  string
	Business  string
	Days      []ExportDay
	Orders    []ExportOrder
	Lines     []ExportLine
	Stock     []ExportStock
	Moves     []ExportMove
	Truncated bool
}

// window matches a created_at column against the shop-local days from $2 to $3 (inclusive), zone $1.
func window(col string) string {
	return fmt.Sprintf(`%[1]s >= ($2::date::timestamp AT TIME ZONE $1::text)
	   AND %[1]s <  (($3::date + 1)::timestamp AT TIME ZONE $1::text)`, col)
}

func (r *PGRepository) ExportData(ctx context.Context, tz string, from, to time.Time) (*ExportData, error) {
	tid, err := tenancy.Require(ctx)
	if err != nil {
		return nil, err
	}
	db := r.db.DB(ctx)
	d := &ExportData{}

	// A shop that never opened its settings has no row yet: fall back to the account name.
	if err := db.QueryRow(ctx,
		`SELECT COALESCE((SELECT business_name FROM settings WHERE tenant_id = $1),
		                 (SELECT display_name FROM users WHERE id = $1))`, tid).Scan(&d.Business); err != nil {
		return nil, err
	}

	rows, err := db.Query(ctx, `
		SELECT (o.created_at AT TIME ZONE $1::text)::date, COUNT(*)::bigint,
		       SUM(o.total_cents - o.tax_cents)::bigint, SUM(o.tax_cents)::bigint, SUM(o.total_cents)::bigint
		  FROM orders o
		 WHERE o.tenant_id = $4::uuid AND o.status = 'completed' AND `+window("o.created_at")+`
		 GROUP BY 1 ORDER BY 1`, tz, from, to, tid)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var x ExportDay
		if err := rows.Scan(&x.Date, &x.Orders, &x.Net, &x.Tax, &x.Gross); err != nil {
			rows.Close()
			return nil, err
		}
		d.Days = append(d.Days, x)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rows, err = db.Query(ctx, `
		SELECT o.receipt_no, (o.created_at AT TIME ZONE $1::text), o.sold_by_name, o.payment_method,
		       o.total_cents - o.tax_cents, o.tax_cents, o.total_cents, o.amount_tendered_cents, o.change_cents
		  FROM orders o
		 WHERE o.tenant_id = $5::uuid AND o.status = 'completed' AND `+window("o.created_at")+`
		 ORDER BY o.created_at, o.receipt_no LIMIT $4`, tz, from, to, rowLimit+1, tid)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var x ExportOrder
		if err := rows.Scan(&x.ReceiptNo, &x.At, &x.Cashier, &x.Payment, &x.Net, &x.Tax, &x.Total, &x.Tendered, &x.Change); err != nil {
			rows.Close()
			return nil, err
		}
		d.Orders = append(d.Orders, x)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rows, err = db.Query(ctx, `
		SELECT o.receipt_no, (o.created_at AT TIME ZONE $1::text), oi.product_name, oi.quantity,
		       oi.unit_price_cents, oi.line_total_cents
		  FROM order_items oi JOIN orders o ON o.id = oi.order_id
		 WHERE o.tenant_id = $5::uuid AND oi.tenant_id = $5::uuid
		   AND o.status = 'completed' AND `+window("o.created_at")+`
		 ORDER BY o.created_at, o.receipt_no, oi.id LIMIT $4`, tz, from, to, rowLimit+1, tid)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var x ExportLine
		if err := rows.Scan(&x.ReceiptNo, &x.At, &x.Product, &x.Qty, &x.Unit, &x.LineTotal); err != nil {
			rows.Close()
			return nil, err
		}
		d.Lines = append(d.Lines, x)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rows, err = db.Query(ctx, `
		SELECT p.sku, p.name, COALESCE(c.name, ''), p.price_cents, p.stock_qty, p.low_stock_threshold, p.is_active
		  FROM products p LEFT JOIN categories c ON c.id = p.category_id
		 WHERE p.tenant_id = $2::uuid
		 ORDER BY p.name LIMIT $1`, rowLimit+1, tid)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var x ExportStock
		if err := rows.Scan(&x.SKU, &x.Name, &x.Category, &x.Price, &x.Qty, &x.Threshold, &x.Active); err != nil {
			rows.Close()
			return nil, err
		}
		d.Stock = append(d.Stock, x)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rows, err = db.Query(ctx, `
		SELECT (m.created_at AT TIME ZONE $1::text), p.name, m.delta, m.reason, COALESCE(s.name, u.display_name)
		  FROM stock_movements m
		  JOIN products p ON p.id = m.product_id
		  LEFT JOIN cashier_seats s ON s.id = m.seat_id
		  LEFT JOIN users u ON u.id = m.user_id
		 WHERE m.tenant_id = $5::uuid AND `+window("m.created_at")+`
		 ORDER BY m.created_at, m.id LIMIT $4`, tz, from, to, rowLimit+1, tid)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var x ExportMove
		if err := rows.Scan(&x.At, &x.Product, &x.Delta, &x.Reason, &x.By); err != nil {
			rows.Close()
			return nil, err
		}
		d.Moves = append(d.Moves, x)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(d.Orders) > rowLimit {
		d.Orders, d.Truncated = d.Orders[:rowLimit], true
	}
	if len(d.Lines) > rowLimit {
		d.Lines, d.Truncated = d.Lines[:rowLimit], true
	}
	if len(d.Stock) > rowLimit {
		d.Stock, d.Truncated = d.Stock[:rowLimit], true
	}
	if len(d.Moves) > rowLimit {
		d.Moves, d.Truncated = d.Moves[:rowLimit], true
	}
	return d, nil
}

// ---- workbook ----

type column struct {
	title string
	width float64
	style int // 0 = none
}

type book struct {
	f                      *excelize.File
	head, title, rp, stamp int
}

func cell(col, row int) string {
	name, _ := excelize.CoordinatesToCellName(col, row)
	return name
}

// table writes a header row, the data rows, column widths and per-column number formats.
func (b *book) table(sheet string, startRow int, cols []column, rows [][]any) error {
	titles := make([]any, len(cols))
	for i, c := range cols {
		titles[i] = c.title
	}
	if err := b.f.SetSheetRow(sheet, cell(1, startRow), &titles); err != nil {
		return err
	}
	if err := b.f.SetCellStyle(sheet, cell(1, startRow), cell(len(cols), startRow), b.head); err != nil {
		return err
	}
	for i := range rows {
		if err := b.f.SetSheetRow(sheet, cell(1, startRow+1+i), &rows[i]); err != nil {
			return err
		}
	}
	for i, c := range cols {
		name, err := excelize.ColumnNumberToName(i + 1)
		if err != nil {
			return err
		}
		if err := b.f.SetColWidth(sheet, name, name, c.width); err != nil {
			return err
		}
		if c.style != 0 && len(rows) > 0 {
			if err := b.f.SetCellStyle(sheet, cell(i+1, startRow+1), cell(i+1, startRow+len(rows)), c.style); err != nil {
				return err
			}
		}
	}
	return nil
}

var paymentLabel = map[string]string{"cash": "Tunai", "card": "Kartu", "transfer": "Transfer", "qris": "QRIS"}
var reasonLabel = map[string]string{"sale": "Penjualan", "restock": "Restock", "correction": "Koreksi", "damage": "Rusak/hilang"}

func yesNo(b bool) string {
	if b {
		return "Ya"
	}
	return "Tidak"
}

// rupiah converts minor units (x100) to whole rupiah for the sheet.
func rupiah(minor int64) int64 { return minor / 100 }

func writeWorkbook(w io.Writer, d *ExportData) error {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	b := &book{f: f}

	var err error
	if b.head, err = f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "064E3B"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"D1FAE5"}, Pattern: 1},
		Alignment: &excelize.Alignment{Vertical: "center", WrapText: true},
	}); err != nil {
		return err
	}
	if b.title, err = f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Size: 14}}); err != nil {
		return err
	}
	rpFmt := `"Rp" #,##0`
	if b.rp, err = f.NewStyle(&excelize.Style{CustomNumFmt: &rpFmt}); err != nil {
		return err
	}
	stampFmt := "dd/mm/yyyy hh:mm"
	if b.stamp, err = f.NewStyle(&excelize.Style{CustomNumFmt: &stampFmt}); err != nil {
		return err
	}

	// 1. Daily summary
	const sumSheet = "Ringkasan Harian"
	if err := f.SetSheetName("Sheet1", sumSheet); err != nil {
		return err
	}
	_ = f.SetCellValue(sumSheet, "A1", d.Business)
	_ = f.SetCellStyle(sumSheet, "A1", "A1", b.title)
	_ = f.SetCellValue(sumSheet, "A2", fmt.Sprintf("Laporan penjualan %s sampai %s", d.From, d.To))
	if d.Truncated {
		_ = f.SetCellValue(sumSheet, "A3", fmt.Sprintf("Catatan: lembar lain dipotong di %d baris. Persempit rentang tanggal untuk data lengkap.", rowLimit))
	}
	var tOrders, tNet, tTax, tGross int64
	days := make([][]any, 0, len(d.Days)+1)
	for _, x := range d.Days {
		days = append(days, []any{x.Date.Format("02/01/2006"), x.Orders, rupiah(x.Net), rupiah(x.Tax), rupiah(x.Gross)})
		tOrders += x.Orders
		tNet += x.Net
		tTax += x.Tax
		tGross += x.Gross
	}
	days = append(days, []any{"Total", tOrders, rupiah(tNet), rupiah(tTax), rupiah(tGross)})
	if err := b.table(sumSheet, 5, []column{
		{"Tanggal", 14, 0}, {"Transaksi", 12, 0}, {"Penjualan sebelum pajak", 24, b.rp}, {"Pajak", 16, b.rp}, {"Total penjualan", 18, b.rp},
	}, days); err != nil {
		return err
	}

	// 2. Transactions
	if _, err := f.NewSheet("Transaksi"); err != nil {
		return err
	}
	orders := make([][]any, 0, len(d.Orders))
	for _, o := range d.Orders {
		orders = append(orders, []any{
			fmt.Sprintf("%06d", o.ReceiptNo), o.At, o.Cashier, paymentLabel[o.Payment],
			rupiah(o.Net), rupiah(o.Tax), rupiah(o.Total), rupiah(o.Tendered), rupiah(o.Change),
		})
	}
	if err := b.table("Transaksi", 1, []column{
		{"No. struk", 12, 0}, {"Waktu", 18, b.stamp}, {"Kasir", 20, 0}, {"Pembayaran", 13, 0},
		{"Sebelum pajak", 16, b.rp}, {"Pajak", 14, b.rp}, {"Total", 16, b.rp}, {"Uang diterima", 16, b.rp}, {"Kembalian", 14, b.rp},
	}, orders); err != nil {
		return err
	}

	// 3. Items sold
	if _, err := f.NewSheet("Item Terjual"); err != nil {
		return err
	}
	lines := make([][]any, 0, len(d.Lines))
	for _, l := range d.Lines {
		lines = append(lines, []any{fmt.Sprintf("%06d", l.ReceiptNo), l.At, l.Product, l.Qty, rupiah(l.Unit), rupiah(l.LineTotal)})
	}
	if err := b.table("Item Terjual", 1, []column{
		{"No. struk", 12, 0}, {"Waktu", 18, b.stamp}, {"Produk", 34, 0}, {"Jumlah", 10, 0}, {"Harga satuan", 16, b.rp}, {"Total", 16, b.rp},
	}, lines); err != nil {
		return err
	}

	// 4. Current stock
	if _, err := f.NewSheet("Stok Saat Ini"); err != nil {
		return err
	}
	stock := make([][]any, 0, len(d.Stock))
	for _, s := range d.Stock {
		status := "Aman"
		if s.Qty <= 0 {
			status = "Habis"
		} else if s.Qty <= s.Threshold {
			status = "Menipis"
		}
		stock = append(stock, []any{s.SKU, s.Name, s.Category, rupiah(s.Price), s.Qty, s.Threshold, status, yesNo(s.Active)})
	}
	if err := b.table("Stok Saat Ini", 1, []column{
		{"SKU", 14, 0}, {"Produk", 34, 0}, {"Kategori", 18, 0}, {"Harga", 16, b.rp},
		{"Stok", 10, 0}, {"Batas menipis", 14, 0}, {"Status", 12, 0}, {"Dijual", 10, 0},
	}, stock); err != nil {
		return err
	}

	// 5. Stock history
	if _, err := f.NewSheet("Riwayat Stok"); err != nil {
		return err
	}
	moves := make([][]any, 0, len(d.Moves))
	for _, m := range d.Moves {
		moves = append(moves, []any{m.At, m.Product, m.Delta, reasonLabel[m.Reason], m.By})
	}
	if err := b.table("Riwayat Stok", 1, []column{
		{"Waktu", 18, b.stamp}, {"Produk", 34, 0}, {"Perubahan", 12, 0}, {"Alasan", 16, 0}, {"Oleh", 20, 0},
	}, moves); err != nil {
		return err
	}

	_, err = f.WriteTo(w)
	return err
}
