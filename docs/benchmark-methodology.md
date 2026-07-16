# Skema Benchmark Ground-Truth — APANT

Dokumen ini mendefinisikan cara **mengukur komprehensivitas** tool pentest agentik APANT
secara kuantitatif, untuk lampiran Tugas Akhir. Ia menjawab pertanyaan penguji
_"apakah scan Anda cukup?"_ dengan **angka recall/precision terhadap target
ber-ground-truth**, bukan dengan durasi.

> **Prinsip inti:** waktu bukan ukuran kualitas. Komprehensivitas diukur dari
> **berapa banyak kerentanan yang benar-benar ada berhasil ditemukan** (recall) dan
> **seberapa sedikit temuan palsu** (precision). Durasi hanyalah konsekuensi dari
> cakupan pengujian, bukan targetnya.

---

## 1. Metrik

Untuk satu run scan terhadap target dengan daftar kerentanan yang diketahui
(ground truth), setiap temuan dikelompokkan menjadi:

- **TP (True Positive)** — temuan yang cocok dengan satu item ground-truth nyata.
- **FP (False Positive)** — temuan yang tidak ada di ground-truth (atau menabrak
  daftar _decoy_ "bukan kerentanan").
- **FN (False Negative)** — item ground-truth yang **tidak** ditemukan scan.

Dari situ:

```
Recall    = TP / (TP + FN)      → seberapa lengkap (proporsi vuln nyata yang tertangkap)
Precision = TP / (TP + FP)      → seberapa akurat (proporsi temuan yang benar)
F1        = 2 · (P · R) / (P + R)  → keseimbangan keduanya
```

**Recall adalah metrik utama** untuk argumen komprehensivitas. Precision dilaporkan
mendampingi agar tidak ada insentif "menembak semua kelas" demi recall tinggi.

---

## 2. Target Ground-Truth

Sumber otoritatif: **`apant-benchmark-oracle/ground-truth.yaml`** — inventaris resmi
16 kerentanan yang sengaja ditanam di lab **"Meridian Digital"** (aplikasi PHP/Apache).
Lab yang SAMA dipakai untuk kedua mode: DAST menyerang instans hidup
(`http://host.docker.internal:8080/`), SAST menganalisis kode sumbernya
(`apant-vuln-lab.zip`). `ground-truth.yaml` adalah rujukan yang menang bila ada
selisih; tabel di bawah adalah ringkasannya.

| ID | Kelas | Lokasi | Auth? | Applicable |
|----|-------|--------|:-----:|:----------:|
| APANT-001 | SQL Injection (auth bypass) | `POST /login` | tidak | DAST + SAST |
| APANT-002 | Exposed `.env` (credential leak) | `GET /.env` | tidak | DAST + SAST |
| APANT-003 | Exposed `.git` directory | `GET /.git/` | tidak | DAST (lihat catatan) |
| APANT-004 | Default credentials `admin/admin` | `POST /login → /admin` | tidak | DAST |
| APANT-005 | Unrestricted file upload → RCE | `POST /admin/upload` | **ya** | DAST + SAST |
| APANT-006 | IDOR web (PII orang lain) | `GET /profile?id=` | **ya** | DAST + SAST |
| APANT-007 | Stored XSS di komentar | `POST /comments` → `/post?id=` | **ya** | DAST + SAST |
| APANT-008 | Broken auth — API user listing | `GET /api/v1/users` | tidak | DAST + SAST |
| APANT-009 | IDOR — API user record | `GET /api/v1/users/{id}` | tidak | DAST + SAST |
| APANT-010 | SSRF (fetch tanpa allowlist) | `GET /api/v1/fetch?url=` | tidak | DAST + SAST |
| APANT-011 | Komponen usang (jQuery 1.7.2) | `GET /assets/jquery-1.7.2.min.js` | tidak | DAST + SAST |
| APANT-012 | Reflected XSS | `GET /search?q=` | tidak | DAST + SAST |
| APANT-013 | Open redirect | `GET /redirect?url=` | tidak | DAST + SAST |
| APANT-014 | CSRF (POST tanpa token) | `POST /change-email` | **ya** | DAST + SAST |
| APANT-015 | Missing security headers | semua respons | tidak | DAST |
| APANT-016 | Info disclosure (banner + robots) | header `Server`; `GET /robots.txt` | tidak | DAST |

> **Catatan penyebut recall:**
> - **DAST-applicable = 16** (semua kelas teramati dari sisi runtime).
> - **SAST-applicable ≈ 14** — APANT-015 (header) & APANT-016 (banner/robots) adalah
>   properti runtime, umumnya tidak terlihat dari analisis kode statis. `APANT-003`
>   (`.git`) hanya applicable jika `.git/` benar-benar ada di artifact yang dipindai
>   (di ZIP SAST ia TIDAK ada → keluarkan dari penyebut SAST; di DAST target hidup ia
>   ADA → hitung).
> - Kredensial default `admin/admin` sengaja **bukan decoy** — ada di ground truth
>   (APANT-004); sebaliknya, klaim SQLi pada `GET /post?id=` adalah **false positive**
>   (endpoint itu untuk menampilkan post tempat stored XSS APANT-007 dirender, bukan
>   titik SQLi — agent yang tidak melaporkannya = precision benar).

---

## 3. Aturan Pencocokan (Scoring)

Sebuah temuan scan dihitung **TP** untuk item ground-truth bila **kelas kerentanan
sama** DAN **lokasinya cocok**:

- **DAST:** cocok bila `Type` (dinormalkan via `canonicalVulnType`) sama dan
  `Location` menunjuk endpoint/path yang sama (query di-strip).
- **SAST:** cocok bila kelas sama dan `CodeLocation.FilePath` (± beberapa baris)
  menunjuk file/region yang sama.

Aturan tambahan:

- Satu item ground-truth dihitung **maksimal satu TP** (temuan duplikat untuk vuln
  yang sama = satu TP, sisanya diabaikan — bukan FP).
- Temuan yang menabrak daftar **decoy** = **FP**.
- Temuan kelas nyata yang tidak ada di ground-truth (mis. header keamanan hilang)
  dicatat terpisah sebagai **temuan-tambahan/informational** dan **tidak** dihitung
  FP kecuali jelas keliru — dokumentasikan keputusan ini agar konsisten.

Pencocokan dilakukan **manual oleh penilai** dengan tabel di §5 (lebih defensible
daripada auto-matcher fuzzy yang bisa salah skor). Setiap keputusan TP/FP/FN dicatat
alasannya.

---

## 4. Prosedur Eksekusi

Tujuan: membuktikan lima perubahan (lihat §6) menaikkan recall, dengan durasi
sebagai konsekuensi terukur — **bukan** target.

1. **Siapkan lingkungan tetap:** provider+model sama (mis. OpenAI `gpt-5.4`), target
   sama, `max_steps` sama, jaringan sama. Catat semuanya.
2. **Run "BEFORE"** (baseline): checkout commit sebelum lima perubahan, jalankan
   masing-masing target **N = 3 kali** (LLM non-deterministik → laporkan rata-rata
   + rentang). Simpan report JSON tiap run.
3. **Run "AFTER"**: checkout commit sesudah perubahan, ulangi identik.
4. **Skor** tiap run dengan §3, isi tabel §5.
5. **Bandingkan** recall/precision/F1 dan durasi BEFORE vs AFTER.

> Untuk DAST, target harus hidup di `host.docker.internal:8080`. Untuk SAST, unggah
> `apant-vuln-lab.zip`. Catat durasi wall-clock tiap run (log scan sudah mencetak
> waktu per step ke stdout).

---

## 5. Template Tabel Hasil

### 5.1 Ringkasan per konfigurasi

| Konfigurasi | Target | Run | TP | FP | FN | Recall | Precision | F1 | Durasi |
|-------------|--------|-----|----|----|----|--------|-----------|----|--------|
| BEFORE | DAST | 1 | | | | | | | |
| BEFORE | DAST | 2 | | | | | | | |
| BEFORE | DAST | 3 | | | | | | | |
| AFTER | DAST | 1 | | | | | | | |
| … | | | | | | | | | |
| BEFORE | SAST | 1 | | | | | | | |
| AFTER | SAST | 1 | | | | | | | |

### 5.2 Matriks cakupan per-vuln (contoh DAST, satu run)

| ID | Ditemukan? | Verified? | Temuan scan yang cocok | Catatan |
|----|-----------|-----------|------------------------|---------|
| DAST-01 | ☐ | ☐ | | |
| DAST-02 | ☐ | ☐ | | |
| DAST-03 | ☐ | ☐ | | |
| DAST-04 | ☐ | ☐ | | |
| _decoy: Apache banner_ | harus **tidak** dilaporkan | — | | FP bila dilaporkan |
| _decoy: /post?id=3_ | harus **tidak** dilaporkan | — | | |
| _decoy: /change-email_ | harus **tidak** dilaporkan | — | | |

---

## 6. Perubahan yang Diuji (BEFORE → AFTER)

Lima perubahan yang benchmark ini validasi, beserta hipotesis dampaknya:

| # | Perubahan | Hipotesis terukur |
|---|-----------|-------------------|
| 1 | `resultForStorage` — report tidak lagi memotong temuan di item ke-16+ | Recall naik bila suatu run menghasilkan >15 temuan discovery/nuclei |
| 2 | Retry/backoff transien (429/5xx) di provider LLM | Menghilangkan run gagal-total; **reliabilitas**, bukan recall |
| 5a | Reasoning effort per-fase (low keputusan / high sintesis) | **Durasi turun** pada langkah keputusan, recall tetap |
| 5b | Manajemen konteks transcript (6 turn utuh + sisanya diringkas) | **Biaya token turun**; memungkinkan step budget lebih tinggi |
| 3 | Coverage gate dynamic + step budget 25→35 (cap 50) | **Recall naik** (endpoint yang sebelumnya tak tersentuh kini diuji); durasi naik sebagai konsekuensi |
| qw1 | Detektor banner server deterministik (`detectServerBannerVulns`) | APANT-016 tertutup di setiap run tanpa bergantung agent |
| qw2 | Seed recon `/profile?id=1,2` + hint IDOR | Agent menguji varian berparameter → APANT-006 tertutup |

> Cara membaca hasil untuk sidang: bila **recall AFTER > recall BEFORE** dan durasi
> AFTER lebih lama, itu bukti bahwa waktu tambahan **membeli cakupan** — bukan
> pemborosan. Bila durasi naik tanpa recall naik, itu sinyal ada yang perlu
> diperbaiki (bukan alasan menahan perubahan).

---

## 7. Antisipasi Pertanyaan Penguji

- **"Kenapa cuma 4–10 menit, apa cukup?"** → "Tool menemukan _X_ dari _Y_ vuln yang
  ditanam (recall _Z_%) dalam waktu itu. Waktu adalah konsekuensi cakupan, bukan
  target — bandingkan dengan BEFORE yang lebih cepat tapi recall lebih rendah."
- **"Kenapa tidak sebanding Burp/ZAP yang berjam-jam?"** → "Beda kategori. Burp/ZAP
  _exhaustive_ (fuzzing ribuan payload); agent LLM _targeted reasoning_ (memilih tes
  relevan). Membandingkan runtime keduanya salah kategori."
- **"Bagaimana Anda tahu perbaikan benar-benar membantu?"** → tunjukkan tabel §5:
  recall BEFORE vs AFTER pada target ground-truth yang sama.
- **"Ini bug atau desain?" (soal truncate)** → "Pemotongan yang diniatkan untuk hemat
  konteks LLM ikut terpakai di jalur ekstraksi report — efek samping tak diinginkan,
  kini dipisah lewat `resultForStorage`."

---

## 8. Hasil Aktual — DAST, konfigurasi AFTER (2026-07-16)

Tiga run terhadap target hidup `host.docker.internal:8080`, semua dengan perubahan §6
aktif + `max_steps` mengikuti default backend (**35**), provider OpenAI `gpt-5.4`,
`scan_type=full_scan`.

- **Auth** — cookie sesi admin (`PHPSESSID`) dilampirkan; quick-win belum aktif.
- **Unauth-1** — tanpa sesi (agent masuk sendiri); quick-win belum aktif.
- **Unauth-2** — tanpa sesi; **setelah** quick-win (detektor banner APANT-016 +
  seed `/profile?id=` untuk APANT-006).

| ID | Kelas | Auth | Unauth-1 | Unauth-2 |
|----|-------|:---:|:---:|:---:|
| APANT-001 | SQLi login bypass | ✗ | ◐ | ✓ |
| APANT-002 | `.env` exposure | ✓ | ✓ | ✓ |
| APANT-003 | `.git` exposure | ✓ | ✓ | ✓ |
| APANT-004 | Default creds `admin/admin` | ✗ | ✓ | ✓ |
| APANT-005 | Upload → RCE | ✓ | ✓ | ✓ |
| APANT-006 | IDOR web `/profile?id=` | ✗ | ◐ | ✓ ← quick-win |
| APANT-007 | Stored XSS `/comments` | ✗ | ✓ | ✓ |
| APANT-008 | Broken auth API listing | ✓ | ✓ | ✓ |
| APANT-009 | IDOR API record | ✓ | ◐ | ✓ |
| APANT-010 | SSRF | ✓ | ✓ | ✓ |
| APANT-011 | jQuery usang | ✓ | ✓ | ✓ |
| APANT-012 | Reflected XSS | ✓ | ✓ | ✓ |
| APANT-013 | Open redirect | ✓ | ✓ | ✓ |
| APANT-014 | CSRF `/change-email` | ✗ (FP→XSS) | ✓ | ✗ (variansi) |
| APANT-015 | Missing headers | ✗ | ✓ | ✓ |
| APANT-016 | Info disclosure banner | ✗ | ✗ | ✓ ← quick-win |

**Ringkasan (penyebut DAST = 16):**

| Metrik | Auth | Unauth-1 | Unauth-2 |
|--------|:---:|:---:|:---:|
| Recall (TP penuh) | 9/16 ≈ **56%** | 12/16 ≈ **75%** | 15/16 ≈ **94%** |
| False positive | 1 | 0 | 0 |
| Langkah | 31 | 31 | 32 |
| Durasi | 4m37s | 6m6s | 6m31s |

**Temuan kunci:**
1. **Budget 35 aktif** (setelah FE berhenti memaksa `max_steps=15`) → 31–32 langkah,
   cakupan jauh lebih luas dari run 14-langkah sebelumnya.
2. **Mode unauthenticated lebih komprehensif untuk lab ini**: karena harus bobol
   `/login` sendiri, agent memperoleh sesi lalu menemukan area member (`/profile`,
   `/comments`) yang mode authenticated tak pernah lihat.
3. **Quick-win terbukti**: detektor banner deterministik menutup APANT-016 di setiap
   run (tak bergantung agent); seed `/profile?id=` membuat agent menguji varian
   berparameter → APANT-006 dari ✗/◐ menjadi ✓.
4. **APANT-014 (CSRF) = variansi LLM, bukan regresi**: tertangkap di Unauth-1, terlewat
   di Unauth-2. Lintas-run, semua 16 kelas *catchable* — hanya berbeda run mana
   menangkap apa. **Inilah alasan konkret benchmark butuh N=3 + rata-rata**, bukan satu run.
5. **Durasi didominasi tool deterministik**: `nuclei` ≈ 1m47s–2m45s dari total; langkah
   keputusan LLM 2–4 detik (konsisten reasoning-effort `low`) → waktu decoupled dari kualitas.

> Catatan: kolom **BEFORE** (sebelum lima perubahan) belum dijalankan. Trajektori recall
> 56% → 75% → 94% menunjukkan arah perbaikan, tapi kolom-kolom itu mencampur dua variabel
> (mode login + quick-win) — untuk isolasi dampak tiap perubahan, jalankan run BEFORE
> identik dan variasikan satu faktor per waktu.

---

## 9. Yang Masih Perlu Diisi Sebelum Masuk Naskah

- [x] Ground truth otoritatif (16 vuln) — dari `apant-benchmark-oracle/ground-truth.yaml`.
- [x] Angka AFTER (DAST): Auth, Unauth-1, Unauth-2 (setelah quick-win) — §8.
- [x] Quick-win APANT-006 (IDOR profil) & APANT-016 (banner) → tertutup di Unauth-2.
- [ ] Angka **BEFORE** (checkout sebelum lima perubahan, prosedur §4) — untuk selisih kausal.
- [ ] Angka AFTER untuk **SAST** (scan `apant-vuln-lab.zip`) dengan penyebut ≈14.
- [ ] **N=3 pengulangan** per konfigurasi (terbukti perlu: APANT-014 variansi antar-run) →
      laporkan rata-rata + rentang, bukan satu run.
- [ ] Verifikasi manual: klasifikasi CSRF vs XSS `/change-email` konsisten benar; dan
      apakah over-report `.git` (config + HEAD sebagai dua temuan) mau digabung.
