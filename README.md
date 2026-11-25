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
- [Deploy 2 service di Render](#deploy-2-service-di-render)
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
├── backend/Dockerfile             image API saja (Render Web Service)
├── Dockerfile  docker-compose.yml  .env.example   image all-in-one
├── render.yaml                    Blueprint Render: API + Static Site

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

### Deploy 2 service di Render

Cara lain menjalankan ChloroPOS selain satu image Docker: **API (Go) sebagai Web Service** dan **frontend (Svelte) sebagai Static Site**, dengan database di **Neon**. Image all-in-one di `Dockerfile` root tetap ada dan tidak berubah cara pakainya.

```text
Browser ──► Static Site  (frontend/dist, VITE_API_BASE)
   │
   └──────► Web Service  (backend/Dockerfile, mode API-only) ──► Neon PostgreSQL
              /healthz        cookie sesi + CORS + CSRF
```

Berkas yang terlibat: `backend/Dockerfile` (API saja), `render.yaml` (Blueprint opsional), `frontend/.env.example` dan `.env.example` (daftar variabel).

#### 1. Pilih strategi cookie dulu

Sesi tetap berupa JWT di cookie `httpOnly` (tidak pernah di `localStorage`), CSRF tetap wajib di setiap request selain GET. Yang berbeda hanya atribut `SameSite`, dan itu bergantung pada alamat yang kamu pakai.

`onrender.com` ada di *Public Suffix List*, jadi `chloropos-web.onrender.com` dan `chloropos-api.onrender.com` dianggap **situs yang berbeda** oleh browser (bukan sekadar origin yang berbeda).

| | **A. Alamat bawaan `*.onrender.com`** | **B. Satu domain induk milikmu** (disarankan untuk produksi) |
|---|---|---|
| Alamat | `https://chloropos-web.onrender.com` + `https://chloropos-api.onrender.com` | `https://app.domain.com` + `https://api.domain.com` |
| Cookie | `SameSite=None; Secure; Partitioned` (otomatis, lihat di bawah) | `SameSite=Lax; Secure`, hanya untuk host API |
| Setup | Tanpa DNS | Dua custom domain di Render + dua record CNAME |
| `COOKIE_SAMESITE` | `auto` | `lax` |
| Browser | Chrome dan Edge standar jalan. **Safari (semua browser di iPhone/iPad memakai mesin yang sama) dan browser yang memblokir cookie pihak ketiga kemungkinan besar menolak cookie-nya**: login "berhasil" lalu langsung terlempar. Aplikasi mendeteksinya dan menampilkan pesan khusus. | Semua browser |
| Cocok untuk | Demo, staging, uji coba | Dipakai kasir sungguhan (HP kasir sering iPhone) |

Mode `auto` bekerja per request: cookie `SameSite=None` hanya diberikan kalau `APP_ENV=production`, `COOKIE_SECURE=true`, dan request datang dari origin yang **terdaftar di `ALLOWED_ORIGINS`** serta berbeda dari origin API. Selain itu (lokal, same-origin, origin tak dikenal) cookie tetap `SameSite=Strict`. Request logout memakai atribut yang sama, supaya cookie benar-benar terhapus.

Untuk strategi B, biarkan `COOKIE_DOMAIN` kosong. Cookie tanpa `Domain` hanya milik `api.domain.com`, dan tetap terkirim dari `app.domain.com` karena keduanya satu situs. Mengisi `.domain.com` justru membuat cookie terlihat oleh semua subdomain.

#### 2. Langkah deploy (urutan: database → API → Static Site)

**2.1 Neon (database)**

1. Buat project di Neon. Database bawaannya biasanya bernama `neondb`.
2. Di **SQL Editor**, jalankan ini sebagai pemilik database. Ini membuat role khusus aplikasi yang **tidak** melewati row-level security (role bawaan Neon biasanya melewatinya, dan isolasi antar toko hanya tinggal bergantung pada filter di query). Ganti nama database dan password (huruf dan angka saja, karena ikut masuk URL):

```sql
CREATE ROLE chloropos_app WITH LOGIN PASSWORD 'GANTI_DENGAN_HURUF_ANGKA';
GRANT CONNECT, CREATE ON DATABASE neondb TO chloropos_app;
GRANT USAGE, CREATE ON SCHEMA public TO chloropos_app;

-- Opsional, jaring pengaman tambahan. Aplikasi sudah mengirim batas waktu ini saat menyambung,
-- tapi saya tidak bisa memastikan Neon menerapkannya untuk parameter startup.
ALTER ROLE chloropos_app SET statement_timeout = '15s';
ALTER ROLE chloropos_app SET idle_in_transaction_session_timeout = '30s';
```

3. Cek: `SELECT rolname, rolsuper, rolbypassrls FROM pg_roles WHERE rolname = 'chloropos_app';` harus menghasilkan `false` dan `false`. Setelah API pertama kali jalan, log-nya **tidak boleh** memuat peringatan *"the database role bypasses row-level security"*.
4. Ambil connection string **direct**, yaitu host **tanpa** `-pooler` (di dialog Connect, matikan *Connection pooling*), lalu ganti user dan password dengan `chloropos_app`:

```text
postgresql://chloropos_app:PASSWORD@ep-xxxxxx.REGION.aws.neon.tech/neondb?sslmode=require
```

Kenapa direct: aplikasi mengirim `statement_timeout` sebagai parameter startup, dan migrasi memakai advisory lock tingkat sesi. Keduanya bermasalah lewat PgBouncer (mode transaksi), yang dipakai endpoint `-pooler`. Mulailah dari database kosong; jangan menjalankan aplikasi dengan role pemilik bawaan lebih dulu, karena tabel yang terbuat akan dimiliki role itu.

**2.2 Render: Web Service (API)**

*Lewat Blueprint (paling cepat):* **New → Blueprint**, pilih repo ini, lalu isi nilai yang diminta (lihat 2.4 untuk dua URL). Lanjut ke 2.4.

*Manual:* **New → Web Service**, repo ini, lalu:

| Pengaturan | Nilai |
|---|---|
| Language / Runtime | Docker |
| Dockerfile Path | `./backend/Dockerfile` |
| Docker Build Context Directory | `./backend` |
| Health Check Path | `/healthz` |
| Instance type | `Free` untuk coba-coba; paid untuk dipakai sungguhan dan untuk scale |
| Environment | Isi seperti tabel API di bagian 3. **Jangan isi `STATIC_DIR`.** |

(Nama label di dashboard bisa sedikit berbeda.) Tunggu deploy selesai, lalu buka `https://<nama-api>.onrender.com/healthz`: harus kosong dengan status `204`. Catat alamat API-nya.

**2.3 Render: Static Site (frontend)**

**New → Static Site**, repo ini, lalu:

| Pengaturan | Nilai |
|---|---|
| Build Command | `cd frontend && npm ci && npm run build` |
| Publish Directory | `frontend/dist` |
| Environment | `VITE_API_BASE` = alamat API tadi (`https://<nama-api>.onrender.com`, tanpa `/` di akhir dan tanpa `/api/v1`) dan `NODE_VERSION` = `22` |
| Redirects/Rewrites | `/*` → `/index.html` (**Rewrite**), supaya `/stok` dan halaman lain tidak 404 saat di-refresh |

`VITE_API_BASE` dibaca saat **build** dan tertanam di hasil build. Mengubahnya berarti deploy ulang Static Site. Kalau nilainya salah bentuk (mis. memuat `/api/v1`), build berhenti dengan pesan yang jelas. Build juga menaruh `Content-Security-Policy` di `index.html` dengan `connect-src` yang hanya membuka alamat API itu; `render.yaml` menambahkan header lain (`X-Frame-Options` dan seterusnya).

**2.4 Hubungkan keduanya**

Alamat Static Site dipakai API, dan alamat API dipakai Static Site, jadi salah satunya harus diisi setelah yang lain ada. Kalau kamu memakai Blueprint, isi dugaan alamat dulu (`https://chloropos-web.onrender.com` dan `https://chloropos-api.onrender.com`, dengan syarat nama itu belum dipakai orang lain; kalau sudah, Render menambah akhiran acak). Setelah dua service terbit:

1. Salin alamat **sebenarnya** Static Site, set sebagai `ALLOWED_ORIGINS` di API (Environment → Save; Render deploy ulang otomatis).
2. Pastikan `VITE_API_BASE` di Static Site sama dengan alamat API sebenarnya; kalau kamu mengubahnya, **Manual Deploy → Clear build cache & deploy**.

**2.5 Admin pertama**

Isi `BOOTSTRAP_ADMIN_USERNAME` dan `BOOTSTRAP_ADMIN_PASSWORD` (minimal 12 karakter) pada deploy pertama saja. Buka alamat Static Site, masuk sebagai admin, lalu **hapus kedua variabel itu** di Environment API.

#### 3. Daftar variabel per service

**API (Web Service)**

| Variabel | Wajib | Nilai |
|---|---|---|
| `DATABASE_URL` | ya | String koneksi Neon **direct** untuk role `chloropos_app` (2.1) |
| `JWT_SECRET` | ya | Acak, minimal 32 karakter. Blueprint membuatkannya. Manual: `openssl rand -base64 48`. **Semua instance harus memakai nilai yang sama**; mengganti nilainya mengeluarkan semua orang. |
| `APP_ENV` | ya | `production` (tanpa ini, mode `auto` tidak pernah memberi `SameSite=None`) |
| `ALLOWED_ORIGINS` | ya | Origin Static Site, persis: `https://chloropos-web.onrender.com`. Pisahkan dengan koma kalau lebih dari satu. Tanpa `/` di akhir, tanpa path, tanpa `*` (aplikasi menolak start kalau salah bentuk). |
| `COOKIE_SECURE` | tidak | `true` (default) |
| `COOKIE_SAMESITE` | tidak | `auto` (default; strategi A) atau `lax` (strategi B) |
| `COOKIE_DOMAIN` | tidak | Kosongkan |
| `REPORT_TZ` | tidak | `Asia/Jakarta` (default), `Asia/Makassar`, `Asia/Jayapura` |
| `BOOTSTRAP_ADMIN_USERNAME`, `BOOTSTRAP_ADMIN_PASSWORD` | deploy pertama | Hapus setelah admin ada |
| `DB_MAX_CONNS`, `DB_MIN_CONNS` | tidak | `10` dan `2` **per instance** |
| `SESSION_HOURS` | tidak | `10` |
| `PORT` | otomatis | Diisi Render. Di production, server mendengarkan di `PORT` kalau `HTTP_ADDR` kosong. |
| `STATIC_DIR` | **jangan diisi** | Kosong = mode API-only |

**Static Site**

| Variabel | Wajib | Nilai |
|---|---|---|
| `VITE_API_BASE` | ya | Origin API, mis. `https://chloropos-api.onrender.com` (waktu build) |
| `NODE_VERSION` | disarankan | `22` |

Untuk strategi B, ubah tiga nilai: `ALLOWED_ORIGINS=https://app.domain.com`, `COOKIE_SAMESITE=lax`, `VITE_API_BASE=https://api.domain.com`. Custom domain ditambahkan di tiap service (**Settings → Custom Domains**); Render menampilkan record CNAME yang harus dibuat di DNS-mu, lalu menerbitkan sertifikat TLS sendiri setelah terverifikasi.

#### 4. Checklist debug

**Health check.** `curl -i https://<api>/healthz` harus `204`. `503` berarti API hidup tapi tidak bisa menjangkau Neon (cek `DATABASE_URL`). Tidak menjawab lama di paket Free berarti service sedang bangun dari tidur (sekitar satu menit); layar "Memuat…" akan menunggu selama itu.

**Error CORS di console** (`blocked by CORS policy`).
- `ALLOWED_ORIGINS` harus sama persis dengan yang tampil di address bar browser: skema, host, dan port, tanpa `/` di akhir. `http` dan `https` berbeda; `www.` dan tanpa `www.` berbeda.
- Cek preflight dari terminal (harus 2xx dan memuat dua header `access-control-*` di bawah):

```bash
curl -i -X OPTIONS https://<api>/api/v1/auth/login \
  -H "Origin: https://<static-site>" \
  -H "Access-Control-Request-Method: POST" \
  -H "Access-Control-Request-Headers: content-type,x-csrf-token"
# access-control-allow-origin: https://<static-site>
# access-control-allow-credentials: true
```

- Tidak ada header itu: origin tidak terdaftar, atau API belum dideploy ulang setelah `ALLOWED_ORIGINS` diubah.

**Cookie tidak tersimpan** (login berhasil, tapi langsung keluar, atau muncul pesan soal cookie sesi).
- DevTools → Network → request `login` → *Response Headers*. Untuk strategi A harus ada `set-cookie: pos_session=...; HttpOnly; Secure; SameSite=None; Partitioned`. Kalau `SameSite=Strict`, salah satunya belum benar: `APP_ENV=production`, `COOKIE_SECURE=true`, atau `ALLOWED_ORIGINS` tidak memuat origin itu persis.
- Header sudah benar tapi cookie tidak muncul di *Application → Cookies* (ikon peringatan di request): browser memblokir cookie pihak ketiga. Pakai strategi B, atau uji di Chrome standar.
- Frontend memanggil `/api/...` di alamatnya sendiri, bukan API: `VITE_API_BASE` kosong saat build. Ubah, lalu deploy ulang Static Site. (Aplikasi menampilkan "Alamat server tidak menjawab seperti API ChloroPOS" untuk kasus ini.)

**403.** Lihat kolom `error.code` di respons:
- `bad_origin`: origin pengirim tidak ada di `ALLOWED_ORIGINS`.
- `bad_csrf`: token CSRF tidak cocok dengan sesi. Biasanya akun lain masuk di tab lain pada browser yang sama; muat ulang halaman. Kalau terjadi terus, pastikan `X-CSRF-Token` ikut terkirim (tanpa itu error-nya berupa CORS, bukan 403).
- `forbidden`: peran akun tidak berhak untuk endpoint itu.

**401 terus-menerus.** Cookie tidak terkirim (lihat atas), atau `JWT_SECRET` berbeda antar instance, atau berubah sejak login.

**429 `rate_limited`.** Lihat catatan scale di bawah.

#### 5. Scale: yang sudah siap dan yang belum

Siap: API tidak menyimpan sesi di memori. Token ditandatangani `JWT_SECRET` yang sama di semua instance, dan tiap request divalidasi ke database. Migrasi memakai advisory lock, jadi beberapa instance yang start bersamaan aman.

Perlu kamu ketahui sebelum menambah instance:

- **Rate limit tersimpan di memori per instance.** Dengan N instance, batas efektifnya sekitar N kali lipat.
- **IP klien.** Di balik proxy Render, `RemoteAddr` adalah IP proxy, bukan IP pengguna. Kode sengaja mengabaikan `X-Forwarded-For` (klien bisa memalsukannya, dan menurut laporan pengguna Render hanya menambahkan isinya tanpa membersihkan nilai kiriman klien). Akibatnya batas per-IP pada login dan API terbagi di antara semua pengguna di balik proxy yang sama, sehingga beberapa kasir yang masuk bersamaan bisa terkena `429`. Batas per-username tetap bekerja normal. Ini belum diubah karena butuh keputusan soal proxy mana yang dipercaya.
- **Koneksi database.** Total koneksi = jumlah instance × `DB_MAX_CONNS` (default 10). Pastikan di bawah batas koneksi paket Neon-mu.
- **Admin pertama.** Buat admin dengan satu instance. Dua instance yang start bersamaan pada deploy pertama bisa berebut membuatnya; yang kalah berhenti lalu dijalankan ulang Render, dan itu tidak berbahaya.
- **Neon dan `/healthz`.** Health check menyentuh database, jadi selama Render memanggilnya, compute Neon tidak sempat tidur dan terus memakai jatah jam komputasi. Periksa kuota paket Neon-mu.
- **Paket Free di Render** tidur kalau sepi dan hanya boleh satu instance. Untuk scale, pakai paket berbayar lalu naikkan `numInstances` (atau jumlah instance di dashboard).

#### 6. Mencoba di lokal

Sebelum deploy, jalankan `cd backend && go vet ./... && go test ./...` dan `cd frontend && npm test`.

**a. Dev biasa (proxy Vite, tidak ada yang berubah).** `VITE_API_BASE` kosong:

```powershell
cd backend ; go run ./cmd/api        # :8080
cd frontend ; npm run dev            # :5173, meneruskan /api ke :8080
```

Buka `http://localhost:5173`, masuk sebagai pemilik dan sebagai kasir. Di tab Network, request ke `http://localhost:5173/api/v1/...` (same-origin).

**b. Frontend dan API di origin berbeda (mode API-only + CORS + CSRF).** Simulasi Static Site dengan build produksi:

```powershell
# terminal 1: API-only (STATIC_DIR kosong), mengizinkan origin preview
cd backend
$env:ALLOWED_ORIGINS = "http://localhost:4173" ; go run ./cmd/api

# terminal 2: build dengan alamat API lalu sajikan
cd frontend
$env:VITE_API_BASE = "http://localhost:8080" ; npm run build ; npm run preview
```

Buka `http://localhost:4173`. Yang harus terlihat: request ke `http://localhost:8080/api/v1/...` dengan preflight `OPTIONS` berstatus 2xx, tanpa error CORS di console, login berhasil, request berikutnya membawa cookie dan `X-CSRF-Token`. (`localhost:4173` dan `localhost:8080` satu situs, jadi cookie tetap `Strict` dan ini tidak menguji `SameSite=None`.)

**c. Opsional: memicu `SameSite=None` di lokal.** Pakai nama host yang berbeda supaya dianggap lintas-situs: API dengan `APP_ENV=production`, `COOKIE_SECURE=true`, `ALLOWED_ORIGINS=http://localhost:4173`; frontend dengan `VITE_API_BASE=http://127.0.0.1:8080`; buka `http://localhost:4173`. Header login harus memuat `SameSite=None; Secure; Partitioned`. Langkah ini bergantung pada perilaku browser untuk cookie `Secure` di `http://` dan belum dicoba.

---

### Pajak dan struk

* Pengaturan: nyalakan pajak, isi nama (PPN) dan tarif, pilih **harga sudah termasuk pajak** (umum untuk retail) atau **pajak ditambahkan di atas harga**.
* Dihitung server dan dibulatkan ke rupiah penuh; tarif dan caranya disalin ke tiap transaksi, jadi mengubah tarif tidak mengubah struk lama.
* Struk mencetak nama usaha, badan usaha, alamat, telepon, email, NPWP, nomor struk, waktu, nama kasir, item, subtotal/pajak/total, metode bayar, kembalian, dan pesan penutup. Lebar cetak 72 mm untuk printer thermal.
* Ini alat bantu, bukan nasihat pajak. Tarif dan kewajiban nota atau faktur pajak usahamu sebaiknya dipastikan ke konsultan pajak.

### Export Excel

Ringkasan, lalu **Unduh laporan Excel**. Berkas berisi lima lembar: Ringkasan Harian, Transaksi, Item Terjual, Stok Saat Ini, Riwayat Stok. Rentang maksimal 1 tahun dan 50.000 baris per lembar; kalau terpotong, ada catatan di lembar pertama.

### Keamanan singkat

**Isolasi antar toko:** tenant diambil dari sesi yang ditandatangani (bukan dari URL atau isi permintaan), setiap query memfilter `tenant_id`, setiap transaksi database dikunci ke tenant lewat row-level security, dan kunci asing gabungan memastikan produk, kategori, kasir, dan transaksi tidak bisa menyilang antar toko. Jatah kasir dijaga dengan kunci baris di database, jadi dua permintaan serentak tidak bisa sama-sama mengambil kursi terakhir. Perubahan akun dan kasir dicatat di log (`audit`: siapa, apa, siapa yang diubah; tanpa password atau kode).

Argon2id, JWT di cookie `httpOnly` + `Secure` + `SameSite=Strict` (di production, untuk frontend lintas-origin yang terdaftar di `ALLOWED_ORIGINS`, otomatis `SameSite=None; Secure; Partitioned`; lihat [Deploy 2 service di Render](#deploy-2-service-di-render)), token CSRF di dalam sesi, cek Origin, query berparameter, rate limit login per IP dan per username, header keamanan (CSP tanpa inline), batas ukuran body, dan harga/pajak selalu dihitung server. Sesi divalidasi ke database tiap permintaan (cache 10 detik), jadi akun yang dinonaktifkan atau di-reset langsung tidak berlaku.

Kalau kasir dibuka lewat `http://192.168.x.x` di LAN, cookie Secure tidak tersimpan. Pasang HTTPS di depannya (Caddy paling mudah) atau set `COOKIE_SECURE=false` dengan sadar bahwa sesi lalu lewat jaringan tanpa enkripsi.

### Belum ada

Void dan refund, diskon, tata letak khusus printer thermal 58 mm, antrean offline, ganti password sendiri oleh pemilik (admin bisa reset di Pengguna), pembayaran jatah kasir otomatis (admin mengaturnya manual), dan menghapus kasir (hanya bisa dinonaktifkan).

---

[⬆️ Kembali ke atas / Back to top](#bahasa-indonesia) | [🇬🇧 Switch to English](#english)

---
<a name="english"></a>
## 🇬🇧 English

**C**ellix **H**elping **O**utlet **R**eady **O**nline **POS**. A lightweight point-of-sale application for home businesses and small offices (SOHO). Go + PostgreSQL on the backend, Svelte + Tailwind on the frontend, compiled into a single binary in production. Indonesian UI, Rupiah (IDR) currency.

> **Status:** written without a compiler in reach (the build sandbox had no Go toolchain or network). Expect to fix a few small things on the first `go build` / `npm run build`. The sales use-case tests (including tax) run with `go test ./internal/sales/...`.

### Table of Contents

* [Roles](https://www.google.com/search?q=%23roles)
* [Layout](https://www.google.com/search?q=%23layout)
* [Run on Windows (no Docker)](https://www.google.com/search?q=%23run-on-windows-no-docker)
* [Upgrading from an earlier version](https://www.google.com/search?q=%23upgrading-from-an-earlier-version)


* [Docker (optional)](https://www.google.com/search?q=%23docker-optional)
* [Deploy as two services on Render](#deploy-as-two-services-on-render)
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
├── backend/Dockerfile             API-only image (Render Web Service)
├── Dockerfile  docker-compose.yml  .env.example   all-in-one image
├── render.yaml                    Render Blueprint: API + Static Site

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

### Deploy as two services on Render

A second way to run ChloroPOS besides the single Docker image: the **Go API as a Render Web Service** and the **Svelte frontend as a Render Static Site**, with the database on **Neon**. The all-in-one `Dockerfile` at the repository root is unchanged. The full step-by-step guide, written in Indonesian, is in [Deploy 2 service di Render](#deploy-2-service-di-render). In short:

* **API:** `backend/Dockerfile` (API only; leave `STATIC_DIR` empty), health check `/healthz`, listens on Render's `PORT` in production.
* **Static Site:** build `cd frontend && npm ci && npm run build`, publish `frontend/dist`, rewrite `/*` to `/index.html`, and build-time `VITE_API_BASE=https://your-api-origin` (origin only: no trailing slash, no `/api/v1`).
* **Cookies, pick one:** (A) default `*.onrender.com` URLs, where the two services are different *sites* (`onrender.com` is on the Public Suffix List), so `COOKIE_SAMESITE=auto` sends `SameSite=None; Secure; Partitioned` to origins listed in `ALLOWED_ORIGINS`. Safari and browsers that block third-party cookies will likely refuse it. Or (B, recommended for production) one parent domain (`app.domain.com` + `api.domain.com`) with `COOKIE_SAMESITE=lax`, which works in every browser. The cookie stays `httpOnly`, there is no token in `localStorage`, and CSRF is still required on every non-GET request.
* **Neon:** use the **direct** connection string (host without `-pooler`) for a dedicated role that does not bypass row-level security (SQL in the Indonesian guide).
* **Order:** database, then API, then Static Site; finally set `ALLOWED_ORIGINS` on the API to the Static Site's real origin.

| Service | Variables |
|---|---|
| API | `DATABASE_URL`, `JWT_SECRET` (same on every instance), `APP_ENV=production`, `ALLOWED_ORIGINS`, optional `COOKIE_SECURE`, `COOKIE_SAMESITE`, `COOKIE_DOMAIN`, `REPORT_TZ`, `DB_MAX_CONNS`, first deploy only `BOOTSTRAP_ADMIN_*`; do not set `STATIC_DIR` |
| Static Site | `VITE_API_BASE` (build time), `NODE_VERSION=22` |

`render.yaml` is a Blueprint for both services. Known limits when scaling out: rate limits are in memory per instance, and behind Render's proxy every user shares the proxy's IP for per-IP limits (details in the Indonesian guide).

---

### Tax and receipts

* Settings: enable tax, enter the tax name (e.g., PPN) and rate, and choose **tax-inclusive pricing** (common in retail) or **tax added on top of price**.
* Calculated server-side and rounded to whole Rupiah; the tax rate and calculation mode are copied onto each transaction, so changing rates later does not alter historical receipts.
* Receipts print business name, legal entity, address, phone, email, tax ID (NPWP), receipt number, timestamp, cashier name, line items, subtotal/tax/total, payment method, change due, and a footer note. Formatted for 72 mm thermal printers.
* This is a bookkeeping utility, not tax advice. Confirm applicable tax rates and invoice/receipt compliance requirements for your business with a tax consultant.

### Excel Export

Go to Summary, then click **Download Excel Report**. The workbook contains five sheets: Daily Summary, Transactions, Items Sold, Current Stock, and Stock History. Maximum date range is 1 year and 50,000 rows per sheet; if truncated, a notice is added to the first sheet.

### Security overview

**Tenant isolation:** the tenant ID is resolved from the signed session (never from URLs or request bodies), every query filters by `tenant_id`, every database transaction is scoped to the tenant via row-level security, and composite foreign keys prevent products, categories, cashiers, and transactions from crossing tenant boundaries. Cashier seat limits are guarded with database row locks so concurrent requests cannot claim the last seat simultaneously. Account and cashier changes are recorded in an audit log (`audit`: who, what, target user; passwords and codes are never logged).

Argon2id password hashing, JWT in `httpOnly` + `Secure` + `SameSite=Strict` cookies (in production, for a cross-origin frontend listed in `ALLOWED_ORIGINS`, automatically `SameSite=None; Secure; Partitioned`; see [Deploy as two services on Render](#deploy-as-two-services-on-render)), session-bound CSRF tokens, `Origin` header validation, parameterized SQL queries, per-IP and per-username login rate limiting, strict security headers (CSP with no inline scripts/styles), body size limits, and server-side price/tax calculation. Sessions are validated against the database on each request (10-second cache), ensuring deactivated or reset accounts lose access immediately.

If the POS is accessed over plain HTTP on a local LAN (e.g., `http://192.168.x.x`), `Secure` cookies will not be stored by browsers. Either place HTTPS in front of it (Caddy is the easiest option) or set `COOKIE_SECURE=false` with the understanding that session cookies will travel unencrypted over the local network.

### Not built yet

Voids and refunds, discounts, dedicated 58 mm thermal printer layout, offline queuing, owner self-service password changes (admins can reset passwords in Users), automated billing for cashier seats (admins adjust seats manually), and permanent deletion of cashiers (can only be deactivated).