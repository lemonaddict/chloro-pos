package catalog

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"chloropos/internal/platform/apperr"
	"chloropos/internal/platform/database"
	"chloropos/internal/platform/tenancy"
)

var (
	errProductNotFound = apperr.NotFound("product_not_found", "Produk tidak ditemukan.")
	// LIKE wildcards typed by the user are matched literally.
	likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
)

// Every query below filters by the tenant from the context, on top of the row-level security the
// database applies: a shop only ever sees and changes its own catalog.
const productSelect = `SELECT p.id, p.category_id, c.name, p.sku, p.barcode, p.name, p.price_cents,
	p.stock_qty, p.low_stock_threshold, p.is_active
	FROM products p LEFT JOIN categories c ON c.id = p.category_id `

type PGRepository struct{ db *database.Store }

func NewPGRepository(db *database.Store) *PGRepository { return &PGRepository{db: db} }

type scanner interface{ Scan(dest ...any) error }

func scanProduct(row scanner) (Product, error) {
	var p Product
	err := row.Scan(&p.ID, &p.CategoryID, &p.CategoryName, &p.SKU, &p.Barcode, &p.Name,
		&p.PriceCents, &p.StockQty, &p.LowStockThreshold, &p.IsActive)
	p.StockStatus = stockStatus(p.StockQty, p.LowStockThreshold)
	return p, err
}

func (r *PGRepository) ListProducts(ctx context.Context, f ProductFilter) ([]Product, error) {
	tid, err := tenancy.Require(ctx)
	if err != nil {
		return nil, err
	}
	// Every value is a bound parameter. The ILIKE pattern is escaped, and the exact barcode/SKU
	// match lets a scanner hit one product instantly.
	rows, err := r.db.DB(ctx).Query(ctx, productSelect+`
		WHERE p.tenant_id = $8::uuid
		  AND ($1::text = ''
		       OR p.name ILIKE '%' || $2::text || '%' ESCAPE '\'
		       OR p.barcode = $1::text
		       OR lower(p.sku) = lower($1::text))
		  AND ($3::uuid IS NULL OR p.category_id = $3::uuid)
		  AND ($4::bool OR p.is_active)
		  AND (NOT $7::bool OR p.stock_qty <= p.low_stock_threshold)
		ORDER BY CASE WHEN $7::bool THEN p.stock_qty END, p.name, p.id
		LIMIT $5::int OFFSET $6::int`,
		f.Search, likeEscaper.Replace(f.Search), f.CategoryID, f.IncludeInactive, f.Limit, f.Offset, f.LowStock, tid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Product, 0, f.Limit)
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *PGRepository) GetProduct(ctx context.Context, id uuid.UUID) (*Product, error) {
	tid, err := tenancy.Require(ctx)
	if err != nil {
		return nil, err
	}
	p, err := scanProduct(r.db.DB(ctx).QueryRow(ctx, productSelect+`WHERE p.id = $1 AND p.tenant_id = $2`, id, tid))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errProductNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *PGRepository) CreateProduct(ctx context.Context, in ProductInput) (*Product, error) {
	tid, err := tenancy.Require(ctx)
	if err != nil {
		return nil, err
	}
	var id uuid.UUID
	err = r.db.DB(ctx).QueryRow(ctx,
		`INSERT INTO products (tenant_id, category_id, sku, barcode, name, price_cents, low_stock_threshold, is_active)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, COALESCE($8::bool, TRUE))
		 RETURNING id`,
		tid, in.CategoryID, in.SKU, in.Barcode, in.Name, in.PriceCents, in.LowStockThreshold, in.IsActive).Scan(&id)
	if err != nil {
		return nil, mapWriteErr(err)
	}
	return r.GetProduct(ctx, id)
}

func (r *PGRepository) UpdateProduct(ctx context.Context, id uuid.UUID, in ProductInput) (*Product, error) {
	tid, err := tenancy.Require(ctx)
	if err != nil {
		return nil, err
	}
	tag, err := r.db.DB(ctx).Exec(ctx,
		`UPDATE products
		    SET category_id = $2, sku = $3, barcode = $4, name = $5, price_cents = $6,
		        low_stock_threshold = $7, is_active = COALESCE($8::bool, is_active), updated_at = now()
		  WHERE id = $1 AND tenant_id = $9`,
		id, in.CategoryID, in.SKU, in.Barcode, in.Name, in.PriceCents, in.LowStockThreshold, in.IsActive, tid)
	if err != nil {
		return nil, mapWriteErr(err)
	}
	if tag.RowsAffected() == 0 {
		return nil, errProductNotFound
	}
	return r.GetProduct(ctx, id)
}

// AdjustStock changes stock and records the movement. Call it inside a transaction (the use
// case does) so the two writes succeed or fail together. The WHERE clause keeps stock >= 0
// even when two people adjust the same product at once. Only owners adjust stock, so actor is
// the owner's user ID.
func (r *PGRepository) AdjustStock(ctx context.Context, id uuid.UUID, delta int, reason string, actor uuid.UUID) (*Product, error) {
	tid, err := tenancy.Require(ctx)
	if err != nil {
		return nil, err
	}
	db := r.db.DB(ctx)
	tag, err := db.Exec(ctx,
		`UPDATE products SET stock_qty = stock_qty + $2, updated_at = now()
		  WHERE id = $1 AND tenant_id = $3 AND stock_qty + $2 >= 0`, id, delta, tid)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		if _, err := r.GetProduct(ctx, id); err != nil {
			return nil, err // not found
		}
		return nil, apperr.Conflict("stock_below_zero", "Stok tidak boleh kurang dari nol.")
	}
	if _, err := db.Exec(ctx,
		`INSERT INTO stock_movements (tenant_id, product_id, delta, reason, user_id) VALUES ($1, $2, $3, $4, $5)`,
		tid, id, delta, reason, actor); err != nil {
		return nil, err
	}
	return r.GetProduct(ctx, id)
}

func (r *PGRepository) ListCategories(ctx context.Context) ([]Category, error) {
	tid, err := tenancy.Require(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.DB(ctx).Query(ctx, `SELECT id, name FROM categories WHERE tenant_id = $1 ORDER BY lower(name)`, tid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Category, 0, 16)
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *PGRepository) CreateCategory(ctx context.Context, name string) (*Category, error) {
	tid, err := tenancy.Require(ctx)
	if err != nil {
		return nil, err
	}
	c := Category{Name: name}
	err = r.db.DB(ctx).QueryRow(ctx, `INSERT INTO categories (tenant_id, name) VALUES ($1, $2) RETURNING id`, tid, name).Scan(&c.ID)
	if n, ok := database.UniqueViolation(err); ok && n == "categories_tenant_name_key" {
		return nil, apperr.Conflict("category_exists", "Kategori itu sudah ada.")
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func mapWriteErr(err error) error {
	if name, ok := database.UniqueViolation(err); ok {
		switch name {
		case "products_tenant_sku_key":
			return apperr.Conflict("sku_taken", "SKU itu sudah dipakai produk lain.")
		case "products_tenant_barcode_key":
			return apperr.Conflict("barcode_taken", "Barcode itu sudah dipakai produk lain.")
		}
	}
	if database.IsForeignKeyViolation(err) {
		return apperr.Invalid("bad_category", "Kategori itu tidak ditemukan.")
	}
	return err
}
