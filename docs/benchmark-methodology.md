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
- [ ] **Studi Model × Harness-Assist (target deploy)** — protokol lengkap di **§10**.
      Empat pengaman wajib sebelum run: (a) definisi _ghost_ umum, (b) penilaian _blind_
      dengan rubrik pra-registrasi, (c) **floor** sebagai metrik primer, (d) N untuk sel
      gpt juga. Belum dijalankan.

---

## 10. Protokol Studi Model × Harness-Assist (target deploy) — versi diperketat

Studi **terpisah** dari §6 (yang menguji lima perubahan kualitas di target **docker**).
Pertanyaan studi ini: **seberapa besar perbaikan harness (auto-login deterministik dkk)
menstabilkan dan menaikkan recall model — terutama model lemah (deepseek) — pada target
deploy black-box.** Dilatarbelakangi investigasi 2026-07-24: sel yang sama (deepseek/deploy)
menghasilkan 6, 7, lalu 9 TP bersih tanpa perubahan kode → variansi besar, dan sebagian
"kenaikan" tercemar _ghost finding_ + FP.

### 10.1 Scope & batas kategori (keputusan: Opsi 2 — DAST black-box murni)

- **Dalam scope:** black-box DAST **melalui Cloudflare, port 443 saja**.
- **Di luar scope (batas KATEGORI, bukan gap harness):** origin bypass via **port 2087**,
  serta **git-dumper / source-recovery / white-box**. Strix unggul sebagian karena kedua hal
  ini (terverifikasi dari repo: sandbox shell Kali + multi-agent + mode white-box). Karena
  itu **Strix diposisikan sebagai _reference ceiling_ (SOTA di related-work), BUKAN target
  yang diklaim disamai.** Residu ini diakui sebagai batas metodologi, jangan dijanjikan tutup.
- **Penyebut recall (deploy) = 15**, bukan 16: **APANT-016** (banner Apache/PHP) ter-mask
  Cloudflare di port 443 (dan 2087 pun menampilkan nginx) → tandai **N/A-environment**,
  keluarkan dari penyebut, laporkan terpisah sebagai batas environment.
- **Penyebut HARUS seragam lintas SEMUA yang masuk tabel — termasuk Strix.** Bahaya:
  memberi Strix skor atas 16 sementara tool sendiri atas 15 = diam-diam menghadiahi Strix satu
  poin yang tak bisa kamu raih, mencemari klaim "ceiling". Aturan: bila Strix dikutip dengan
  angka apa pun, ia **di-skor ulang pada permukaan identik** — 15 kelas, 443-only — dengan
  **membuang temuan Strix yang di luar scope Opsi 2** (yang hanya via port 2087, via git-dumper/
  source, dan APANT-016). Kalau tidak di-skor ulang begitu, Strix **tetap kualitatif tanpa angka
  di tabel komparatif** (§10.9) — jangan pernah adu 15-vs-16.

### 10.2 Sel eksperimen

Matriks **{deepseek-v4-pro, gpt-5.4} × {RAW, ASSISTED}** = 4 sel. **N run per sel, termasuk
sel gpt** (syarat d): tanpa varians baseline gpt, klaim _"assist menaikkan model lemah ke
konsistensi model kuat"_ tak bisa dibuat.

- **RAW** = harness sekarang (catat commit hash).
- **ASSISTED** = harness + bundel perbaikan (catat commit hash): auto-login default-cred
  deterministik + coverage-gate diperluas ke kelas non-param (auth/POST-based/SSRF) + dedup
  aksi + fingerprint client-side (APANT-011). **Catatan scope:** fix prompt-nmap untuk
  port-scan **tidak** masuk bundel ini (port 2087 di luar Opsi 2); nmap hanya boleh untuk
  recon di 443.
- Setiap kondisi diuji pada **kedua** model, dengan **max_steps tetap = 35** (terbukti tidak
  binding: run berhenti sukarela di 23/28/33 dari 35 — lihat log 2026-07-24; jangan naikkan
  kecuali, setelah gate diperluas, run mulai menyentuh 35).

### 10.3 Ukuran N

Spread deepseek teramati (6/7/9 TP) besar → N=3 memberi interval kepercayaan lebar, mean dua
kondisi bisa tumpang tindih meski efek nyata. Karena itu:
- **Default N = 5 untuk SEMUA sel, termasuk gpt.** Jangan kunci asimetri "gpt cukup N=3"
  berdasarkan asumsi "gpt stabil" — **kestabilan gpt justru yang mau diukur** (sirkular kalau
  diasumsikan). Kalau ternyata gpt juga high-variance, N=3 sudah terlanjur kecil dan baru
  ketahuan setelah run terpakai (mahal untuk diulang).
- **Bila ingin menghemat run gpt**, boleh berhenti di N=3 HANYA lewat aturan adaptif yang
  ditetapkan di depan: *"lanjut ke N=5 bila range recall bersih gpt pada N=3 > 2 kelas."*
  Aturan itu, bukan asumsi, yang membenarkan N lebih kecil.
- Asimetri N apa pun yang akhirnya dipakai **wajib dijustifikasi eksplisit di naskah** (penguji
  akan menanyakannya).
- Ini aman karena **metrik primer = floor** (§10.7), yang jauh lebih tahan N kecil daripada
  mean. Lebih banyak run hanya memperkuat klaim floor.

### 10.4 Reset state target — WAJIB sebelum SETIAP run (anti-kontaminasi)

Target deploy **stateful lintas scan** (tidak seperti container docker fresh). Sebelum tiap run:
1. **Reset lab ke seed bersih** (di host lab, dir `docker-compose.yml`). Fakta setup: `app`
   tanpa volume (→ `/uploads` fresh saat container di-recreate) dan `db` MySQL pakai named
   volume `apant-db-data` (→ **HANYA `down -v`** yang menghapusnya sehingga `init.sql` re-seed;
   `restart`/`--force-recreate` TIDAK reset DB). Init MySQL dari volume kosong lambat + ada
   `depends_on: service_healthy`, jadi satu `down -v && up -d` sering membatalkan app/api/caddy
   karena db belum sehat. **Sequence andal — start db dulu, tunggu sehat, baru sisanya:**
   ```bash
   docker compose down -v
   docker compose up -d db
   until [ "$(docker inspect -f '{{.State.Health.Status}}' apant-db)" = healthy ]; do sleep 3; done
   docker compose up -d
   docker compose ps   # pastikan caddy/app/api/db semua Up
   # WAJIB: tunggu target PUBLIK benar-benar melayani (app/api warm-up → 502 transien setelah reset).
   # Berhenti di "db healthy" saja TIDAK cukup — pernah 502 lalu 200.
   until [ "$(curl -s -o /dev/null -w '%{http_code}' https://apantlab.pinkmonkeys.web.id/)" = 200 ]; do sleep 3; done
   ```
   > Catatan: `curl http://127.0.0.1:8080/` bisa `000` — itu bukan indikator; scanner menembak
   > URL publik (via tunnel Cloudflare), jadi **gate kesiapan = URL publik balas 200**.
2. **Verifikasi kondisi bersih & catat buktinya:** `GET /uploads/` kosong/403; tidak ada
   stored-XSS di `/post`; tabel user = seed. Checklist ini disimpan per run.
3. **Kunci & rekam konfigurasi Cloudflare** (mode proxy, WAF ruleset, security level, port
   yang di-expose) — HARUS sama untuk semua run; catat tanggal + setting. (Paritas run Strix
   07-20 tak bisa diverifikasi retroaktif → **jangan campur data Strix lama** dengan run baru.)

### 10.5 Aturan validitas temuan (memperketat §3)

Tambahan atas §3, ditetapkan **sebelum** run:

- **Aturan _self-produced_ (definisi ghost UMUM, bukan cuma POST-check):** sebuah temuan
  dihitung TP hanya bila **transkrip run itu sendiri berisi request yang MENGHASILKAN atau
  MEMBUKTIKAN artefaknya.** Bila bukti bergantung pada perubahan state yang tidak diciptakan
  run ini di transkripnya → **BUKAN TP** (tandai `ghost/unverified`, dihitung sebagai
  tidak-ditemukan). Contoh gagal:
  - web shell diakses (`GET /uploads/x.php?c=`) **tanpa** POST upload di run ini → artefak run lain;
  - stored-XSS teramati **tanpa** POST `/comments` yang menanamnya di run ini.
  Reset (10.4) mencegah sumbernya; aturan ini menjaring sisa yang lolos (mis. artefak DB).
- **FP:** sesuai §2/§3 (contoh terkunci: SQLi `GET /post?id=` = FP).
- **Out-of-scope-surface:** temuan yang hanya terjangkau via port 2087 / source code =
  **excluded** (Opsi 2), bukan TP dan bukan FP.
- **Dedup:** banyak temuan → satu VULN-ID = satu TP (`.git/config` + `.git/HEAD` → APANT-003).

### 10.6 Penilaian _blind_ dengan rubrik pra-registrasi (memperketat §3)

- **Pra-registrasi kriteria matching per VULN-ID SEBELUM run** (tabel terkunci): untuk tiap
  dari 15 kelas in-scope, tetapkan **(path/URL yang diterima) + (kelas CWE) + (bukti minimum
  di transkrip untuk TP)**. Ini mencegah penilaian ad-hoc yang bias (ingat kasus deepseek
  melipat 008+009).
- **De-label sebelum menilai:** setelah semua run selesai, lepas label sel (model / raw-vs-
  assisted) dari report+transkrip → beri ID acak → nilai TP/FP/ghost terhadap rubrik **tanpa
  tahu selnya** → baru pasang label kembali untuk agregasi. Menghindari bias penilai (lebih
  murah hati ke run assisted).
- Idealnya penilai kedua; solo → minimal de-label + rubrik tetap. Catat alasan tiap keputusan.

#### 10.6.1 Rubrik matching terkunci (15 kelas in-scope; 016 = N/A-environment)

Tetapkan **sebelum** run apa pun; jangan diubah setelah melihat hasil. "Bukti minimum"
menerapkan aturan _self-produced_ (§10.5): yang diberi tanda **[SP]** wajib punya request
penanam/pembukti di transkrip run itu sendiri, kalau tidak = `ghost`, bukan TP.

| ID | Path/URL diterima | CWE | Bukti minimum di transkrip untuk TP |
|----|-------------------|-----|-------------------------------------|
| 001 SQLi login bypass | `POST /login` | CWE-89 | Run POST **payload SQLi** (mis. `admin' OR '1'='1'-- `) ke `/login` TANPA kredensial valid **DAN** memperoleh sesi authenticated (302→/admin atau konten dashboard). Beda dari 004: payload, bukan `admin/admin` asli. |
| 002 `.env` | `GET /.env` | CWE-200 | Run GET `/.env` → HTTP 200, body memuat secret (`DB_*`/`APP_KEY`/`ADMIN_*`). |
| 003 `.git` | `GET /.git/HEAD`\|`/.git/config` | CWE-200 | Run GET path `.git` → 200 berisi metadata git. (config+HEAD → **1** TP, dedup.) |
| 004 Default creds | `POST /login admin/admin` | CWE-798 | Run POST `admin/admin` → authenticated (302→/admin atau dashboard). |
| 005 Upload→RCE | `POST /admin/upload` (auth) | CWE-434 | **[SP]** Run **POST upload** ke `/admin/upload` **DAN** mengeksekusi shell terunggah (mis. `?c=id`→output `uid=`). Tanpa POST upload di transkrip = ghost. |
| 006 Profile IDOR | `GET /profile?id=` (auth) | CWE-639 | Run (authenticated) GET `/profile?id=N` (id bukan miliknya) → PII user lain. |
| 007 Stored XSS | `POST /comments`→`GET /post?id=` | CWE-79 | **[SP]** Run **POST payload script** ke `/comments` **DAN** GET post menampilkan payload tak ter-sanitasi. Tanpa POST penanam = ghost. |
| 008 Broken-auth API | `GET /api/v1/users` (no sesi) | CWE-200 | Run GET `/api/v1/users` **tanpa sesi** → record user dikembalikan. |
| 009 IDOR API | `GET /api/v1/users/{id}` | CWE-639 | Run GET **≥2 id berbeda** → data user berbeda (ubah id → user lain). |
| 010 SSRF | `GET /api/v1/fetch?url=` | CWE-918 | Run `fetch?url=` menunjuk resource internal/loopback **DAN** respons internal dikembalikan. |
| 011 jQuery usang | HTML/JS klien | CWE-1104 | Run mengidentifikasi **jQuery 1.7.2** dari `<script src>` / isi JS (fingerprint client-side). |
| 012 Reflected XSS | `GET /search?q=` | CWE-79 | Run kirim payload ke `/search?q=` → terrefleksi **tak ter-encode** di konteks eksekusi (konfirmasi dalfox/manual). |
| 013 Open redirect | `GET /redirect?url=` | CWE-601 | Run `/redirect?url=<eksternal>` → **3xx** `Location` ke URL eksternal itu. |
| 014 CSRF | `POST /change-email` (auth) | CWE-352 | Run menunjukkan `/change-email` menerima POST state-changing **tanpa token CSRF/SameSite** (bukti: absennya token **DAN** POST diterima). |
| 015 Missing headers | semua respons | CWE-693 | Run mengamati respons tanpa header keamanan / flag cookie (HttpOnly/Secure/SameSite absen; tak ada CSP/HSTS). |

> **016 (banner Apache/PHP)** sengaja TIDAK di tabel: N/A-environment (§10.1), keluar dari penyebut 15.

### 10.7 Metrik — FLOOR sebagai PRIMER (eksplisit)

- **PRIMER — floor per kelas (hit-rate lintas run):** untuk tiap VULN-ID, laporkan **k/N run
  yang menemukannya**. Klaim inti berbentuk floor, mis. *"cabang auth (004/005/006): RAW
  muncul di 1/5 run; ASSISTED 5/5 run."* Floor tahan N kecil dan tidak tumpang tindih seperti
  mean±range.
- **Pecah DUA sumber varians (jangan digabung — koreksi framing "flip = satu POST"):**
  - **Auth-branch floor:** k/N untuk 004, 005, 006, 007, 014 (butuh sesi/POST). Target auto-login.
  - **Unauth-breadth floor:** k/N untuk 010 (SSRF) dll — kelas unauth yang bergantung keluasan
    eksplorasi. Target coverage-gate + dedup, **BUKAN** auto-login.
- **SEKUNDER — recall mean ± range** (TP **bersih** per run = setelah buang ghost+FP+dup).
  Dilaporkan tapi eksplisit sekunder; bukan headline (N kecil → rentang tumpang tindih).
- **SEKUNDER — verified-recall (kedalaman bukti, mendampingi recall):** dari kelas TP yang
  PUNYA tier-proof bermakna, fraksi yang artefaknya benar-benar **TER-EKSTRAK**, bukan sekadar
  di-flag. `verified-recall = |TP proof-tier| / |TP pada kelas ber-tier-proof|`. Ia mengukur
  **perilaku tool** (apakah mengekstrak bukti), bukan mewarisi tag `[SP]` rubrik — divalidasi
  lewat kode: di detektor exposed-file yang sama, `.env` dengan secret → proof, `/.git/HEAD` →
  flag (deteksi, bukan ekstraksi), sehingga angkanya bergerak mengikuti perilaku, bukan tautologis.
  - **Tier-proof per kelas:** 001 = sesi authenticated diperoleh; 002 = nilai secret ter-baca;
    003 = objek/file `.git` ter-recover (deteksi `/.git/HEAD` = **flag**, plafon jujur tanpa
    object-dump multi-request — _known ceiling + known path_); 005 = output shell (`uid=`);
    008 = baris PII dikembalikan; 010 = respons internal dikembalikan; 013 = 3xx `Location`
    (proof by nature).
  - **Penyebut MENGECUALIKAN kelas observasional-murni** (011 versi, 015 header, 016 banner):
    tak punya tier-proof; memasukkannya mengempeskan metrik secara palsu. 014 (CSRF) inheren
    **flag-tier** (bukti negatif — lihat §10.9), bukan kegagalan ekstraksi.
- **Precision** = TP/(TP+FP) per run, mean ± range. **Ghost dihitung sebagai bukan-TP**, dan
  bila dilaporkan sebagai temuan → masuk FP/excluded, tidak pernah TP.
- **Varians** = spread (min–max) recall bersih per sel.

### 10.8 Template tabel

**10.8.1 Floor per-kelas (metrik primer)** — isi `k/N`:

| VULN-ID | Sumber | deepseek RAW | deepseek ASSISTED | gpt RAW | gpt ASSISTED |
|---------|--------|:---:|:---:|:---:|:---:|
| 004 default creds | auth | | | | |
| 005 upload→RCE | auth | | | | |
| 006 profile IDOR | auth | | | | |
| 007 stored XSS | auth | | | | |
| 014 CSRF | auth | | | | |
| 010 SSRF | breadth | | | | |
| … (kelas unauth lain) | breadth | | | | |
| **Auth-branch (avg k/N)** | | | | | |
| **Unauth-breadth (avg k/N)** | | | | | |

**10.8.2 Ringkasan per run** (TP bersih, penyebut 15):

| Sel | Run | TP-bersih | Ghost-excl | FP | Recall | Verified-recall | Precision |
|-----|-----|:---:|:---:|:---:|:---:|:---:|:---:|
| deepseek RAW | 1..N | | | | | | |
| deepseek ASSISTED | 1..N | | | | | | |
| gpt RAW | 1..N | | | | | | |
| gpt ASSISTED | 1..N | | | | | | |

> **Verified-recall** = fraksi TP proof-tier atas TP pada kelas ber-tier-proof (§10.7);
> penyebutnya BUKAN 15 — kecualikan kelas observasional-murni (011/015/016) dan baca 014
> sebagai flag-tier by design.

### 10.9 Ancaman validitas yang WAJIB ditulis di naskah

- **N kecil** → andalkan **floor**, bukan mean±range yang tumpang tindih.
- **Target tunggal** → tidak generalisasi ke aplikasi lain; nyatakan sebagai batasan.
- **Paritas Cloudflare** dikunci ke depan (10.4), tapi run Strix 07-20 tak terverifikasi
  retroaktif → data Strix hanya sebagai _reference ceiling_ kualitatif, tidak diadu langsung
  angka-ke-angka dengan run terkontrol baru.
- **Penilai solo** → dimitigasi de-label + rubrik pra-registrasi (10.6).
- **Strix beda kelas sistem** (shell/multi-agent/white-box) → recall-vs-Strix bukan metrik
  headline; kontribusi diukur dari **delta RAW → ASSISTED pada tool sendiri**.
- **CSRF (014) = bukti negatif, bukan proof.** Ambang TP-nya "POST state-changing diterima
  tanpa token yang terlihat" adalah **proxy**: token bisa berada di header, double-submit
  cookie, atau origin-check yang tak tampak di satu request. Karena itu 014 inheren
  **flag-tier** di verified-recall — dan ini **batas kelas yang dibagi kompetitor**
  (Strix/Pentagi pun melapor CSRF dari absennya token), bukan kelemahan ekstraksi APANT.
  Tulis eksplisit; jangan baca verified-recall 014 sebagai ekstraksi lemah.
- **Deteksi 001 bergantung pada redirect.** SEMUA kanal deterministik auth-bypass (executor
  `isLikelyAuthBypassRedirect`, report-layer `isAuthBypassRedirect`, heuristik boolean-SQLi)
  menuntut `/login` membalas **3xx + `Location`** saat bypass sukses; `successIndicators`
  executor hanya string lab PortSwigger yang tak menyala di lab custom. Bila `/login` membalas
  **200 inline** tanpa `Location`, semua kanal deterministik 001 gelap dan 001 hanya bergantung
  sqlmap + narasi agen. **Wajib diverifikasi pra-run (gate §10.10)**; §8 lama dikumpulkan
  sebelum ini dicek → status 001 = _to-confirm-on-rerun_, bukan aman.

### 10.10 Runbook eksekusi per-run (ulangi untuk tiap dari 20 run)

Sekali per sesi, catat **snapshot config Cloudflare** (mode proxy, security level, WAF) — bila
tak berubah, cukup rujuk.

**Gate pra-batch (sekali, sebelum run pertama — dua verifikasi yang mengunci validitas 001 & precision):**
1. **Perilaku `/login` saat bypass** — kirim `admin' OR '1'='1'-- ` dan periksa respons: **3xx +
   `Location`** (mis. →/admin) atau **200 inline**. 3xx+Location → kanal deterministik 001 hidup
   (dipulihkan fix type-hygiene). 200 inline → SEMUA kanal deterministik 001 gelap: catat sebagai
   temuan environment dan tambah detektor "dashboard 200-inline" sebelum mengukur 001, atau tandai
   001 bergantung sqlmap+narasi di naskah. **Jangan kunci angka 001 sebelum gate ini dijawab.**
2. **Presisi decoy `/post?id=`** — konfirmasi heuristik boolean-SQLi yang kini menyala (fix
   body_length int) TIDAK melaporkan decoy sebagai FP: cek delta true/false pada `/post?id=` < 500
   byte. Unit test sudah mengunci ambangnya; verifikasi ini memastikan perilaku lab nyata sesuai.

Lalu untuk **setiap** run:

1. **Reset** — jalankan sequence §10.4 sampai `https://apantlab.pinkmonkeys.web.id/` balas **200**.
2. **Sanity bersih** (murah): `curl -s -o /dev/null -w '%{http_code}' https://apantlab.pinkmonkeys.web.id/uploads/pwned.php` → harap **404** (tak ada shell sisa).
3. **Catat metadata run** di lembar tersegel: `{blind_id, model, kondisi(RAW/ASSISTED), commit_hash_harness, timestamp}`. `blind_id` = label acak (R-01…R-20); **mapping ini disimpan terpisah** agar penilaian §10.6 benar-benar buta.
   - **RAW** = scan **unauthenticated** (`scan_type=full_scan`, tanpa auth config) → agent memutuskan sendiri apakah POST /login (inilah varians auth-branch yang diukur).
   - **ASSISTED** = harness dengan bundel (auto-login dll) aktif.
4. **Jalankan scan** ke target publik dengan model yang ditentukan; **simpan `scan_id`** yang dikembalikan.
5. **Ekspor artefak** setelah selesai: report JSON + transkrip langkah (dari tabel `scans`/`reports` via `scan_id`) → simpan di bawah `blind_id`, **tanpa** menyebut model/kondisi di isinya.
6. Ulangi. Urutan run boleh diacak antar-sel agar drift target (bila ada) tak sistematis ke satu kondisi.

**Penilaian (setelah semua 20 run):** de-label → nilai tiap `blind_id` terhadap rubrik §10.6.1
(+ aturan ghost/FP §10.5) tanpa tahu selnya → buka mapping → agregasi ke tabel floor §10.8.1
(primer) + ringkasan §10.8.2.

**Batch RAW dulu (10 run):** deepseek ×5 + gpt-5.4 ×5. Ini mengukur floor/varians baseline
tiap model **dan** memvalidasi reset+rubrik sebelum bundel ASSISTED dikoding.
