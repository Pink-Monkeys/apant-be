# Cookie Security Documentation

Dokumen ini menjelaskan implementasi cookie auth saat ini di backend `apant-be`, arti nilai token, dan masa login efektif user.

## 1) Tiga Cookie Utama

Aplikasi menggunakan 3 cookie:

1. `apant_access`
2. `apant_refresh`
3. `apant_csrf`

### `apant_access`
- Isi: JWT (`header.payload.signature`) yang ditandatangani HS256.
- Tujuan: autentikasi request ke protected endpoint.
- Sifat: `HttpOnly`, tidak bisa dibaca JavaScript browser.
- Validasi: middleware `Protected` memverifikasi signature + expiry.

Claims yang diisi server saat ini:
- `sub`: user ID
- `usr`: username
- `role`: role user
- `iat`: issued-at (Unix seconds)
- `exp`: expire time (Unix seconds)

Referensi kode:
- `internal/application/auth/service.go` (`issueTokenPair`)
- `internal/interfaces/http/middleware/auth.go`

### `apant_refresh`
- Isi: token random 32-byte (base64url), BUKAN JWT.
- Tujuan: meminta access token baru ketika `apant_access` expired.
- Sifat: `HttpOnly`.
- Penyimpanan server: hanya hash SHA-256 yang disimpan di DB (`refresh_tokens.token_hash`), bukan raw token.
- Lifecycle: dipakai dengan mekanisme rotation (token lama direvoke, token baru diterbitkan).

Referensi kode:
- `internal/application/auth/service.go` (`generateRefreshToken`, `hashToken`, `RefreshToken`)
- `internal/infrastructure/db/user_repository_postgres.go`

### `apant_csrf`
- Isi: token random 32-byte (base64url), BUKAN JWT.
- Tujuan: proteksi CSRF dengan pola double-submit cookie.
- Sifat: `HttpOnly=false` agar frontend bisa membaca nilainya dan mengirim ke header `X-CSRF-Token`.
- Validasi: untuk request non-GET/HEAD/OPTIONS, middleware membandingkan nilai header vs cookie dengan constant-time compare.

Referensi kode:
- `internal/interfaces/http/handler/auth_handler.go` (`generateCSRFToken`, `setCSRFCookie`)
- `internal/interfaces/http/middleware/csrf.go`

## 2) Apakah Value Random Itu Punya Makna dari JWT?

Jawaban singkat:
- `apant_access`: YA, ini JWT sehingga payload berisi data claim (`sub`, `usr`, `role`, `iat`, `exp`) dan dapat didecode (tanpa secret tetap bisa dibaca payload, tapi tidak bisa dipalsukan signature-nya).
- `apant_refresh`: TIDAK. Ini token acak, tidak membawa claim terstruktur.
- `apant_csrf`: TIDAK. Ini token acak, tidak membawa claim terstruktur.

## 3) Analisis Token Contoh yang Kamu Kirim

Untuk `apant_access`:
- `iat=1779586178` -> `2026-05-24 01:29:38 UTC`
- `exp=1779672578` -> `2026-05-25 01:29:38 UTC`

Selisihnya 24 jam, sesuai konfigurasi default `AUTH_TOKEN_TTL_HOURS=24`.

## 4) Berapa Lama User Tetap Login?

Secara efektif, ada 2 horizon waktu:

1. Access session pendek (autentikasi request langsung)
- Default: 24 jam (`AUTH_TOKEN_TTL_HOURS`).
- Setelah expired, request protected gagal jika belum refresh.

2. Login persistence (tetap bisa "stay signed in")
- Default: 168 jam / 7 hari (`AUTH_REFRESH_TOKEN_TTL_HOURS`).
- Selama refresh token masih valid dan belum direvoke, user bisa terus mendapat access token baru tanpa login ulang.
- Jika refresh token expired/revoked, user harus login ulang (praktis diarahkan ke `/login` oleh frontend).

Catatan penting implementasi saat ini:
- `refresh-token` endpoint tidak menggunakan middleware `Protected`, jadi refresh tetap bisa dilakukan walau access token sudah expired, asalkan `apant_refresh` + CSRF valid.

## 5) Cookie Security Flags Saat Ini

Dari konfigurasi default:
- `SameSite`: `lax`
- `Secure`: `false` (default env)
- `Path`: `/`
- `Domain`: kosong (host-only cookie)

Artinya untuk production WAJIB atur env:
- `AUTH_COOKIE_SECURE=true` (hanya kirim cookie lewat HTTPS)
- `AUTH_COOKIE_SAMESITE=lax` atau `strict` sesuai kebutuhan alur cross-site
- `JWT_SECRET` harus kuat dan tidak default

## 6) Rekomendasi Hardening (Production)

1. Transport & cookie flags
- Selalu HTTPS + `AUTH_COOKIE_SECURE=true`.
- Pertahankan `HttpOnly=true` untuk access/refresh.
- Gunakan `SameSite=Lax` minimum; gunakan `Strict` jika alur UI memungkinkan.

2. Secret management
- Gunakan `JWT_SECRET` panjang, acak, dan rotasi berkala.
- Jangan gunakan default `change-me`.

3. TTL strategy
- Pertimbangkan access token lebih pendek (misal 15-60 menit) + refresh rotation tetap aktif.
- Pertahankan refresh TTL sesuai risk appetite (misal 7 hari, bisa lebih pendek untuk sistem sensitif).

4. CSRF discipline
- Wajib kirim `X-CSRF-Token` untuk semua mutating request (`POST/PUT/PATCH/DELETE`).
- Frontend harus re-fetch CSRF (`GET /api/v1/auth/csrf`) saat cookie hilang/expired.

5. Logout & revocation
- Logout sudah revoke semua refresh token user; pertahankan behavior ini.
- Tambahkan audit log event login/refresh/logout untuk investigasi insiden.

## 7) Ringkasan Jawaban Pertanyaan Kamu

1. "Value random itu ada maknanya?"
- Hanya `apant_access` yang bermakna terstruktur (JWT claims).
- `apant_refresh` dan `apant_csrf` adalah random token tanpa payload claim.

2. "Session berapa lama user tetap masuk?"
- Access token: default 24 jam.
- Stay logged in lewat refresh: default sampai 7 hari selama refresh token valid dan tidak direvoke.
- Setelah refresh expired/revoked, user wajib login ulang.
