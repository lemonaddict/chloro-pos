# ChloroPOS

**Bahasa:** [🇮🇩 Bahasa Indonesia](#bahasa-indonesia) | [🇬🇧 English](#english)

---

<a name="bahasa-indonesia"></a>
## 🇮🇩 Bahasa Indonesia

**C**ellix **H**elping **O**utlet **R**eady **O**nline **POS**. Aplikasi kasir ringan untuk usaha rumahan dan kantor kecil. Backend Go + PostgreSQL, frontend Svelte + Tailwind, satu binary di produksi. Bahasa Indonesia, mata uang Rupiah.

> **Status:** ditulis tanpa compiler (sandbox pembuatnya tidak punya Go dan jaringan). Kemungkinan ada beberapa perbaikan kecil di `go build` / `npm run build` pertama. Tes use case penjualan (termasuk pajak) jalan dengan `go test ./internal/sales/...`.

### Daftar Isi
- [Peran](#peran)
- [Struktur](#struktur)
- [Menjalankan di Windows (tanpa Docker)](#menjalankan-di-windows-tanpa-docker)
  - [Kalau meng-upgrade dari versi sebelumnya](#kalau-meng-upgrade-dari-versi-sebelumnya)
- [Docker (opsional)](#docker-opsional)
- [Pajak dan struk](#pajak-dan-struk)
- [Export Excel](#export-excel)
- [Keamanan singkat](#keamanan-singkat)
- [Belum ada](#belum-ada)

### Peran

Satu aplikasi untuk banyak toko (multi-tenant). Ada tiga jenis akun:

| | Admin (perusahaan) | Pemilik (pelanggan SOHO) | Kasir (milik satu pemilik) |
|---|---|---|---|
| Buat, nonaktifkan, reset password pelanggan, atur jatah kasir | ya | tidak | tidak |
| Melihat data toko pelanggan (stok, penjualan, laporan) | **tidak** | ya, tokonya sendiri | tidak |
| Layar kasir (jual, bayar, cetak struk) | tidak | ya | ya |
| Angka stok per produk | tidak | ya | tidak (hanya "habis" / "menipis") |
| Ringkasan, laporan Excel, stok dan harga, pengaturan usaha | tidak | ya | tidak |
| Buat dan kelola akun kasir sendiri (menu **Akun kasir**) | tidak | ya, sampai jatahnya | tidak |

Satu akun pemilik = satu toko (tenant). Pemilik membuat "kasir" (instance kasir) sampai jatahnya, **2 secara default**; admin menaikkan jatah per pelanggan di halaman **Pengguna** kalau pelanggan membayar lebih. Semua transaksi kasir tercatat di toko pemiliknya, dengan nama kasirnya disalin ke setiap struk.

**Cara masuk.** Halaman Masuk punya dua pilihan. *Pemilik* (juga untuk admin): username + kata sandi. *Kasir*: username toko + kode kasir. Kode kasir dibuat sistem per kasir (bentuknya `7QK2-M49Q-XD2R`), ditampilkan sekali saat dibuat, dan hanya hash-nya yang disimpan. Kata sandi pemilik tidak pernah dipakai kasir, jadi memberi kasir akses tidak memberinya akses ke toko. Kasir berhenti atau kodenya bocor: **Kode baru** (kode lama dan sesinya langsung mati).

Pembatasan dijaga di API, bukan cuma menu yang disembunyikan: respons produk untuk kasir tidak memuat jumlah stok, endpoint data toko menolak admin, dan endpoint laporan/stok/pengaturan/akun kasir menolak kasir. Menonaktifkan akun, kasir, atau seluruh toko, dan reset password atau kode, langsung mengeluarkan orangnya dari semua perangkat (maksimal sekitar 10 detik karena cache singkat).

### Struktur

```text
chloropos/
├── backend/
│   ├── cmd/api/main.go            titik masuk: config, pool, migrasi, server
│   ├── migrations/                0001 skema awal, 0002 pengaturan + pajak + QRIS + sesi
│   └── internal/
│       ├── app/                   merakit modul, memasang route, menyajikan frontend
│       ├── platform/              config, database, auth, middleware, httpx, apperr
│       ├── identity/              akun, login, sesi, manajemen pengguna
│       ├── catalog/               produk, kategori, stok
│       ├── settings/              profil usaha di struk + aturan pajak
│       ├── sales/                 checkout, pajak, struk
│       └── reporting/             ringkasan harian, produk terlaris, export Excel
├── frontend/src/{lib,components,routes}
├── Dockerfile  docker-compose.yml  .env.example

```

### Menjalankan di Windows (tanpa Docker)

Butuh Go 1.23+, Node 20+, dan PostgreSQL 16+.

1. Buat database:
```powershell
psql -U postgres -c "CREATE USER sohopos WITH PASSWORD 'pilih-huruf-dan-angka'"
psql -U postgres -c "CREATE DATABASE sohopos OWNER sohopos"

```


*(Nama database dan user tetap `sohopos` supaya instalasi lama tidak perlu dibuat ulang.)*
2. Salin `.env.example` menjadi `.env` di root proyek, lalu isi `DATABASE_URL`, `JWT_SECRET`, dan `BOOTSTRAP_ADMIN_PASSWORD`.
3. Backend (migrasi jalan otomatis):
```powershell
cd backend
go mod tidy
go test ./internal/sales/...
go run ./cmd/api

```


4. Frontend, di terminal kedua:
```powershell
cd frontend
npm install
npm run dev

```


5. Buka `http://localhost:5173`, masuk sebagai admin (`admin` + password dari `.env`), lalu hapus dua baris `BOOTSTRAP_ADMIN_*` dari `.env`. *(Nama lama `BOOTSTRAP_OWNER_*` masih terbaca kalau dijalankan tanpa Docker; di docker-compose pakai nama baru.)*
6. Langkah pertama: sebagai admin, buka **Pengguna** dan buat akun pelanggan. Lalu keluar dan masuk sebagai pelanggan itu: **Pengaturan** (nama usaha, alamat, telepon, NPWP, pajak), **Akun kasir** (buat kasir), lalu **Stok**.

#### Kalau meng-upgrade dari versi sebelumnya

* Timpa foldernya, **simpan `.env**` dan `go.sum` milikmu. Jalankan `go mod tidy` (menambah pustaka Excel `excelize`).
* Migrasi `0002` dan `0003` jalan sendiri. Semua orang perlu masuk ulang sekali.
* **Migrasi `0003` mengubah aplikasi menjadi multi-tenant.** Semua data lama (produk, kategori, transaksi, pengaturan) pindah ke akun `owner` tertua. Akun kasir lama menjadi kasir milik akun itu dan terkunci sampai pemiliknya menekan **Kode baru** (password lamanya tidak dipakai lagi). Akun `owner` lain yang ada menjadi pelanggan terpisah tanpa data. Nomor struk mulai dihitung per toko dan melanjutkan dari nomor tertinggi. Buat admin lewat `BOOTSTRAP_ADMIN_*` (dibuat kalau belum ada admin).
* **Nama berkas migrasi**: `NNNN_nama.up.sql` menerapkan, `NNNN_nama.down.sql` membatalkan. Aplikasi hanya menjalankan berkas up dan tidak pernah menyentuh down. Berkas lama bernama `0001_schema.sql` tetap dianggap up, dan mengganti namanya menjadi `0001_schema.up.sql` tidak membuatnya jalan dua kali. Untuk rollback 0003, hentikan aplikasi lalu: `psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -1 -f backend/migrations/0003_multitenant.down.sql` (menolak jalan kalau lebih dari satu toko punya data; baca catatan di kepala berkasnya untuk apa yang tidak bisa dipulihkan).
* **Row-level security.** Tabel data toko dilindungi policy per tenant, dengan `FORCE ROW LEVEL SECURITY` supaya berlaku juga untuk pemilik tabel. Policy tidak berlaku untuk superuser atau role dengan `BYPASSRLS`; aplikasi memberi peringatan saat start kalau role-nya begitu. Setiap query di aplikasi juga memfilter tenant sendiri, jadi isolasi tetap jalan, tapi sebaiknya sambungkan aplikasi sebagai role biasa. Akibat `FORCE`: migrasi di masa depan yang membaca atau mengubah isi tabel toko harus menjalankan `SELECT set_config('app.tenant_id', '<uuid>', true)` dulu, atau `ALTER TABLE ... NO FORCE ROW LEVEL SECURITY` selama migrasi.
* Harga harus rupiah penuh. Produk lama dengan harga berpecahan akan ditolak saat disimpan sampai harganya dibulatkan.
* Set `REPORT_TZ` (default `Asia/Jakarta`; WITA `Asia/Makassar`, WIT `Asia/Jayapura`).
* Restart `npm run dev` kalau `vite.config.js` berubah.

### Docker (opsional)

```powershell
copy .env.example .env      # isi POSTGRES_PASSWORD, JWT_SECRET, BOOTSTRAP_ADMIN_PASSWORD
docker compose up --build

```

Buka `http://localhost:8080`. Server Go menyajikan frontend sendiri, tanpa Nginx. PostgreSQL tidak dibuka ke luar jaringan compose.

### Pajak dan struk

* Pengaturan: nyalakan pajak, isi nama (PPN) dan tarif, pilih **harga sudah termasuk pajak** (umum untuk retail) atau **pajak ditambahkan di atas harga**.
* Dihitung server dan dibulatkan ke rupiah penuh; tarif dan caranya disalin ke tiap transaksi, jadi mengubah tarif tidak mengubah struk lama.
* Struk mencetak nama usaha, badan usaha, alamat, telepon, email, NPWP, nomor struk, waktu, nama kasir, item, subtotal/pajak/total, metode bayar, kembalian, dan pesan penutup. Lebar cetak 72 mm untuk printer thermal.
* Ini alat bantu, bukan nasihat pajak. Tarif dan kewajiban nota atau faktur pajak usahamu sebaiknya dipastikan ke konsultan pajak.

### Export Excel

Ringkasan, lalu **Unduh laporan Excel**. Berkas berisi lima lembar: Ringkasan Harian, Transaksi, Item Terjual, Stok Saat Ini, Riwayat Stok. Rentang maksimal 1 tahun dan 50.000 baris per lembar; kalau terpotong, ada catatan di lembar pertama.

### Keamanan singkat

**Isolasi antar toko:** tenant diambil dari sesi yang ditandatangani (bukan dari URL atau isi permintaan), setiap query memfilter `tenant_id`, setiap transaksi database dikunci ke tenant lewat row-level security, dan kunci asing gabungan memastikan produk, kategori, kasir, dan transaksi tidak bisa menyilang antar toko. Jatah kasir dijaga dengan kunci baris di database, jadi dua permintaan serentak tidak bisa sama-sama mengambil kursi terakhir. Perubahan akun dan kasir dicatat di log (`audit`: siapa, apa, siapa yang diubah; tanpa password atau kode).

Argon2id, JWT di cookie `httpOnly` + `Secure` + `SameSite=Strict`, token CSRF di dalam sesi, cek Origin, query berparameter, rate limit login per IP dan per username, header keamanan (CSP tanpa inline), batas ukuran body, dan harga/pajak selalu dihitung server. Sesi divalidasi ke database tiap permintaan (cache 10 detik), jadi akun yang dinonaktifkan atau di-reset langsung tidak berlaku.

Kalau kasir dibuka lewat `http://192.168.x.x` di LAN, cookie Secure tidak tersimpan. Pasang HTTPS di depannya (Caddy paling mudah) atau set `COOKIE_SECURE=false` dengan sadar bahwa sesi lalu lewat jaringan tanpa enkripsi.

### Belum ada

Void dan refund, diskon, tata letak khusus printer thermal 58 mm, antrean offline, ganti password sendiri oleh pemilik (admin bisa reset di Pengguna), pembayaran jatah kasir otomatis (admin mengaturnya manual), dan menghapus kasir (hanya bisa dinonaktifkan).

---

[⬆️ Kembali ke atas / Back to top](https://www.google.com/search?q=%23chloropos) | [🇬🇧 Switch to English](https://www.google.com/search?q=%23english)

---

## 🇬🇧 English

**C**ellix **H**elping **O**utlet **R**eady **O**nline **POS**. A lightweight point-of-sale application for home businesses and small offices (SOHO). Go + PostgreSQL on the backend, Svelte + Tailwind on the frontend, compiled into a single binary in production. Indonesian UI, Rupiah (IDR) currency.

> **Status:** written without a compiler in reach (the build sandbox had no Go toolchain or network). Expect to fix a few small things on the first `go build` / `npm run build`. The sales use-case tests (including tax) run with `go test ./internal/sales/...`.

### Table of Contents

* [Roles](https://www.google.com/search?q=%23roles)
* [Layout](https://www.google.com/search?q=%23layout)
* [Run on Windows (no Docker)](https://www.google.com/search?q=%23run-on-windows-no-docker)
* [Upgrading from an earlier version](https://www.google.com/search?q=%23upgrading-from-an-earlier-version)


* [Docker (optional)](https://www.google.com/search?q=%23docker-optional)
* [Tax and receipts](https://www.google.com/search?q=%23tax-and-receipts)
* [Excel Export](https://www.google.com/search?q=%23excel-export)
* [Security overview](https://www.google.com/search?q=%23security-overview)
* [Not built yet](https://www.google.com/search?q=%23not-built-yet)

### Roles

One multi-tenant application serving multiple shops. There are three account types:

|  | Admin (platform) | Owner (SOHO customer) | Cashier (belongs to one owner) |
| --- | --- | --- | --- |
| Create, deactivate, reset customer passwords, set cashier seats | yes | no | no |
| View customer shop data (stock, sales, reports) | **no** | yes, own shop only | no |
| Sell screen (checkout, payment, print receipt) | no | yes | yes |
| Exact stock counts per product | no | yes | no ("out of stock" / "low" only) |
| Dashboard summary, Excel export, stock & pricing, shop settings | no | yes | no |
| Create and manage own cashier accounts (**Cashier Accounts** menu) | no | yes, up to seat limit | no |

One owner account = one shop (tenant). Owners create "cashiers" (cashier instances) up to their seat allowance, **2 by default**; admins increase seats per customer on the **Users** page when a customer pays for more. All cashier transactions are recorded under their owner's shop, with the cashier's name stamped onto every receipt.

**Signing in.** The Sign-in page offers two modes. *Owner* (also used by admins): username + password. *Cashier*: shop username + cashier code. Cashier codes are system-generated per cashier (formatted like `7QK2-M49Q-XD2R`), shown once upon creation, and stored only as hashes. The owner's password is never shared with cashiers, so granting POS access does not expose shop administration. If a cashier leaves or a code leaks: click **New code** (the old code and active sessions are immediately invalidated).

Restrictions are enforced at the API level, not merely hidden in the UI: product responses for cashiers omit stock quantities, shop data endpoints reject admins, and reporting/stock/settings/cashier-account endpoints reject cashiers. Deactivating an account, cashier, or entire shop—or resetting a password or code—immediately signs the user out across all devices (within ~10 seconds due to a short cache).

### Layout

```text
chloropos/
├── backend/
│   ├── cmd/api/main.go            entry point: config, pool, migrate, serve
│   ├── migrations/                0001 initial schema, 0002 settings + tax + QRIS + sessions
│   └── internal/
│       ├── app/                   composition root: wires modules, mounts routes, serves frontend
│       ├── platform/              config, database, auth, middleware, httpx, apperr
│       ├── identity/              accounts, sign-in, sessions, user management
│       ├── catalog/               products, categories, stock
│       ├── settings/              business profile on receipts + tax rules
│       ├── sales/                 checkout, tax, receipts
│       └── reporting/             daily summary, top-selling products, Excel export
├── frontend/src/{lib,components,routes}
├── Dockerfile  docker-compose.yml  .env.example

```

### Run on Windows (no Docker)

Requires Go 1.23+, Node 20+, and PostgreSQL 16+.

1. Create the database:
```powershell
psql -U postgres -c "CREATE USER sohopos WITH PASSWORD 'choose-letters-and-digits'"
psql -U postgres -c "CREATE DATABASE sohopos OWNER sohopos"

```


*(The database and user names remain `sohopos` so existing installations do not need to be recreated.)*
2. Copy `.env.example` to `.env` in the project root, then fill in `DATABASE_URL`, `JWT_SECRET`, and `BOOTSTRAP_ADMIN_PASSWORD`.
3. Backend (migrations run automatically on start):
```powershell
cd backend
go mod tidy
go test ./internal/sales/...
go run ./cmd/api

```


4. Frontend, in a second terminal:
```powershell
cd frontend
npm install
npm run dev

```


5. Open `http://localhost:5173`, sign in as admin (`admin` + password from `.env`), then remove the two `BOOTSTRAP_ADMIN_*` lines from `.env`. *(Legacy `BOOTSTRAP_OWNER_*` variables are still recognized when running without Docker; `docker-compose` uses the new names.)*
6. First steps: as admin, go to **Users** and create a customer account. Then sign out and sign in as that customer: configure **Settings** (business name, address, phone, tax ID/NPWP, tax rules), **Cashier Accounts** (create cashiers), and **Stock**.

#### Upgrading from an earlier version

* Overwrite the project files, **keeping your `.env**` and `go.sum`. Run `go mod tidy` (adds the `excelize` Excel library).
* Migrations `0002` and `0003` run automatically. Everyone will need to sign in again once.
* **Migration `0003` converts the application to multi-tenant.** All existing data (products, categories, transactions, settings) is assigned to the oldest `owner` account. Existing cashier accounts become cashiers under that owner and remain locked until the owner clicks **New code** (old cashier passwords are no longer used). Any other existing `owner` accounts become separate tenants with empty data. Receipt numbering becomes per-shop and continues from the highest existing number. Create an admin via `BOOTSTRAP_ADMIN_*` (created only if no admin exists yet).
* **Migration filenames**: `NNNN_name.up.sql` applies changes, `NNNN_name.down.sql` reverts them. The app only runs `.up.sql` files and never touches `.down.sql`. The legacy `0001_schema.sql` file is still treated as an up migration, and renaming it to `0001_schema.up.sql` will not cause it to run twice. To roll back `0003`, stop the app and run: `psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -1 -f backend/migrations/0003_multitenant.down.sql` (refuses to run if more than one shop has data; read the file header comments for what cannot be restored).
* **Row-level security.** Shop data tables are protected by per-tenant policies with `FORCE ROW LEVEL SECURITY` so policies also apply to the table owner. Policies do not apply to superusers or roles with `BYPASSRLS`; the app logs a warning on startup if connected with such a role. Every application query also filters by tenant explicitly, so isolation still holds, but connecting as a regular role is recommended. Because of `FORCE`: future migrations reading or modifying shop tables must first execute `SELECT set_config('app.tenant_id', '<uuid>', true)`, or temporarily run `ALTER TABLE ... NO FORCE ROW LEVEL SECURITY` during the migration.
* Prices must be whole Rupiah integers. Legacy products with fractional prices will be rejected on save until rounded.
* Set `REPORT_TZ` (defaults to `Asia/Jakarta`; WITA `Asia/Makassar`, WIT `Asia/Jayapura`).
* Restart `npm run dev` if `vite.config.js` changes.

### Docker (optional)

```powershell
copy .env.example .env      # fill in POSTGRES_PASSWORD, JWT_SECRET, BOOTSTRAP_ADMIN_PASSWORD
docker compose up --build

```

Open `http://localhost:8080`. The Go server serves the built frontend directly without Nginx. PostgreSQL is not exposed outside the compose network.

### Tax and receipts

* Settings: enable tax, enter the tax name (e.g., PPN) and rate, and choose **tax-inclusive pricing** (common in retail) or **tax added on top of price**.
* Calculated server-side and rounded to whole Rupiah; the tax rate and calculation mode are copied onto each transaction, so changing rates later does not alter historical receipts.
* Receipts print business name, legal entity, address, phone, email, tax ID (NPWP), receipt number, timestamp, cashier name, line items, subtotal/tax/total, payment method, change due, and a footer note. Formatted for 72 mm thermal printers.
* This is a bookkeeping utility, not tax advice. Confirm applicable tax rates and invoice/receipt compliance requirements for your business with a tax consultant.

### Excel Export

Go to Summary, then click **Download Excel Report**. The workbook contains five sheets: Daily Summary, Transactions, Items Sold, Current Stock, and Stock History. Maximum date range is 1 year and 50,000 rows per sheet; if truncated, a notice is added to the first sheet.

### Security overview

**Tenant isolation:** the tenant ID is resolved from the signed session (never from URLs or request bodies), every query filters by `tenant_id`, every database transaction is scoped to the tenant via row-level security, and composite foreign keys prevent products, categories, cashiers, and transactions from crossing tenant boundaries. Cashier seat limits are guarded with database row locks so concurrent requests cannot claim the last seat simultaneously. Account and cashier changes are recorded in an audit log (`audit`: who, what, target user; passwords and codes are never logged).

Argon2id password hashing, JWT in `httpOnly` + `Secure` + `SameSite=Strict` cookies, session-bound CSRF tokens, `Origin` header validation, parameterized SQL queries, per-IP and per-username login rate limiting, strict security headers (CSP with no inline scripts/styles), body size limits, and server-side price/tax calculation. Sessions are validated against the database on each request (10-second cache), ensuring deactivated or reset accounts lose access immediately.

If the POS is accessed over plain HTTP on a local LAN (e.g., `http://192.168.x.x`), `Secure` cookies will not be stored by browsers. Either place HTTPS in front of it (Caddy is the easiest option) or set `COOKIE_SECURE=false` with the understanding that session cookies will travel unencrypted over the local network.

### Not built yet

Voids and refunds, discounts, dedicated 58 mm thermal printer layout, offline queuing, owner self-service password changes (admins can reset passwords in Users), automated billing for cashier seats (admins adjust seats manually), and permanent deletion of cashiers (can only be deactivated).

---

[⬆️ Kembali ke atas / Back to top](https://www.google.com/search?q=%23chloropos) | [🇮🇩 Beralih ke Bahasa Indonesia](https://www.google.com/search?q=%23bahasa-indonesia)

```

### Catatan Tambahan:
- **Tombol / Link Interaktif:** Di bagian paling atas (`[🇮🇩 Bahasa Indonesia](#bahasa-indonesia) | [🇬🇧 English](#english)`) dan di bagian bawah masing-masing bahasa terdapat tautan yang langsung melompat ke bagian bahasa yang dipilih saat diklik di GitHub, GitLab, maupun *Markdown viewer* lainnya.
- **Daftar Isi (Table of Contents):** Setiap versi bahasa memiliki daftar isinya sendiri yang mengarah ke sub-bab masing-masing bahasa.
- **Terjemahan Teknis:** Istilah teknis dari README lama Anda (seperti *composition root*, *sales use-case tests*, *single binary in production*, *thermal printers*, dll.) dipertahankan agar konsisten dengan gaya penulisan Anda sebelumnya.

```