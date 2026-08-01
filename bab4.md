# ANALISIS DAN PEMBAHASAN

Bab ini menyajikan hasil implementasi sistem otomasi *web penetration testing*
berbasis kecerdasan buatan (APANT) beserta analisis dan pembahasan atas hasil
pengujiannya. Pembahasan dibagi menjadi tiga bagian besar. Bagian pertama
menguraikan hasil implementasi setiap komponen sistem sesuai rancangan pada BAB
III. Bagian kedua menyajikan hasil pengujian fungsional (*black-box*) untuk
memverifikasi kesesuaian perilaku sistem terhadap kebutuhan fungsional. Bagian
ketiga menyajikan hasil pengujian efektivitas deteksi kerentanan secara
kuantitatif, termasuk analisis perbandingan antar-*tool* dan antar-model LLM,
analisis biaya token, serta pembahasan menyeluruh atas ketercapaian tujuan
penelitian dan keterbatasan sistem.

## 4.1 Hasil Implementasi Sistem

Sistem berhasil diimplementasikan sebagai empat layanan *microservice* yang
diorkestrasi menggunakan Docker Compose, sesuai rancangan arsitektur pada
Subbab 3.2.2. Keempat layanan (frontend, backend API, scanner, dan basis data
PostgreSQL) berjalan sebagai kontainer terpisah dan berkomunikasi melalui
jaringan internal Docker. Tabel 4.1 merangkum status implementasi setiap
komponen terhadap rancangan.

| **No.** | **Komponen** | **Rancangan (BAB III)** | **Status** |
| --- | --- | --- | --- |
| 1. | Layanan Frontend (React + Tanstack) | Subbab 3.2.2 | Terimplementasi |
| 2. | Layanan Backend API (Go + Fiber v3) | Subbab 3.2.2 | Terimplementasi |
| 3. | Layanan Scanner (kontainer terisolasi) | Subbab 3.2.2 | Terimplementasi |
| 4. | Basis Data PostgreSQL 16 | Subbab 3.2.2 | Terimplementasi |
| 5. | Agentic loop (planner–executor–finalizer) | Subbab 3.2.3 | Terimplementasi |
| 6. | Profil pemindaian (6 profil) | Subbab 3.2.4 | Terimplementasi |
| 7. | Alur SAST (ingest ZIP + navigasi kode) | Subbab 3.2.5 | Terimplementasi |
| 8. | Autentikasi JWT + CSRF | Subbab 3.2.6 | Terimplementasi |

Tabel 4.1: Status implementasi komponen sistem

Seluruh kebutuhan fungsional (KF-01 sampai KF-12 pada Tabel 5) berhasil
diwujudkan. Pembahasan pada subbab berikut memfokuskan pada tiga bagian yang
menjadi inti kontribusi penelitian, yaitu agentic loop, lapisan keamanan agen,
dan alur pengujian statis.

### 4.1.1 Implementasi Agentic Loop

Inti sistem adalah agen multi-langkah berpola ReAct (*reasoning–acting*) yang
diimplementasikan sendiri (*custom*) pada layanan backend menggunakan Go. Agen
menjalankan siklus berulang: pada setiap iterasi, LLM (berperan sebagai
*planner*) menerima transkrip tindakan sebelumnya beserta hasilnya
(*observation*) dan mengembalikan satu tindakan berikutnya dalam format JSON
terstruktur. Keluaran JSON inilah — bukan teks bebas — yang memungkinkan
backend memproses niat agen secara terprogram.

Setiap iterasi menghasilkan salah satu dari dua jenis tindakan:

1. `{"action":"tool", ...}` — agen memanggil sebuah *tool* keamanan dengan
   parameter tertentu. Backend (sebagai *executor*) memvalidasi niat tersebut,
   meneruskannya ke layanan scanner, lalu memasukkan hasil eksekusi kembali ke
   transkrip sebagai *observation* untuk iterasi berikutnya.
2. `{"action":"final", ...}` — agen menyatakan pengujian selesai, sehingga
   siklus berhenti.

Siklus dibatasi oleh **anggaran langkah** (*step budget*) agar tidak berjalan
tanpa batas. Ketika anggaran langkah habis atau agen mengembalikan tindakan
`final`, backend memanggil LLM sekali lagi dalam peran *finalizer* untuk
menyusun ringkasan penilaian keamanan, yang kemudian diolah menjadi laporan
terstruktur. Untuk menjaga keandalan, sistem menerapkan **retry terbatas**
ketika keluaran LLM tidak berupa JSON yang valid (KNF-04), sehingga kegagalan
parsing sesekali tidak menggagalkan seluruh pemindaian.

### 4.1.2 Implementasi Lapisan Keamanan Agen

Sebelum sebuah *tool* dieksekusi, niat pemanggilan dari LLM harus melewati empat
lapis penjagaan secara berurutan sebagaimana dirancang pada Subbab 3.2.3.
Keempat lapis tersebut adalah:

1. **Pembatasan profil** — *tool* yang diminta harus termasuk daftar *tool* yang
   diizinkan oleh profil pemindaian yang dipilih (lihat Subbab 4.1.4).
2. **Penegakan cakupan (*scope*)** — *host* target wajib berada di dalam cakupan
   resmi, sekaligus menolak *host* privat/*loopback*. Lapis ini menjadi
   pertahanan terhadap *Server-Side Request Forgery* (SSRF).
3. **Penjagaan *payload*** — membatasi ukuran *body* permintaan agar tidak
   melebihi batas yang ditetapkan.
4. **Validasi kebijakan** — *tool* harus berada dalam *whitelist* dan seluruh
   parameternya valid.

Selain keempat lapis di atas, sistem menerapkan pertahanan terhadap *prompt
injection*: seluruh keluaran *tool* yang dimasukkan kembali ke konteks agen
ditandai sebagai data tak tepercaya (`<observation untrusted="true">`). Dengan
penandaan ini, instruksi berbahaya yang mungkin disisipkan pada halaman target
(misalnya "*abaikan instruksi sebelumnya dan ..."*) diperlakukan sebagai data,
bukan sebagai perintah yang harus dipatuhi agen. Pengujian atas mekanisme ini
disajikan pada UJI-08 dan UJI-09 (Subbab 4.2).

### 4.1.3 Implementasi Alur Pengujian Statis (SAST)

Pada mode SAST, arsip ZIP yang diunggah pengguna diekstraksi ke dalam
*workspace* terisolasi dengan pembatasan jumlah berkas dan ukuran total untuk
mencegah serangan *zip bomb* dan *path traversal*. Agen kemudian menjalankan
*tool* analisis statis (semgrep untuk pola kerentanan kode, gitleaks untuk
*secret* yang bocor, dan OSV untuk dependensi rentan) dipadukan dengan *tool*
navigasi kode (`list_files`, `read_file`, `grep_code`) untuk memvalidasi temuan
sebelum dimasukkan ke laporan. Khusus *tool* navigasi kode, sistem menerapkan
dua gerbang *path containment* sehingga agen hanya dapat membaca berkas di dalam
*workspace* pemindaian yang sah. Laporan akhir memuat lokasi temuan secara
presisi hingga ke berkas dan nomor baris.

### 4.1.4 Implementasi Profil Pemindaian

Keenam profil pemindaian pada Tabel 4 berhasil diimplementasikan. Setiap profil
membatasi daftar *tool* yang boleh dipanggil agen dan menyuntikkan tujuan
spesifik ke dalam *prompt* sistem, sehingga agen tetap fokus pada kelas
kerentanan yang dituju. Verifikasi bahwa pembatasan ini benar-benar ditegakkan
disajikan pada UJI-07 (Subbab 4.2).

## 4.2 Hasil Pengujian Fungsional (Black-box)

Pengujian fungsional dilakukan mengacu pada rancangan skenario pada Tabel 10
(UJI-01 sampai UJI-11). Setiap skenario diuji dengan membandingkan hasil aktual
terhadap hasil yang diharapkan. Ringkasan hasil disajikan pada Tabel 4.2.

| **No** | **Kode** | **Skenario** | **Hasil yang Diharapkan** | **Hasil Aktual** | **Status** |
| --- | --- | --- | --- | --- | --- |
| 1 | UJI-01 | Registrasi pengguna | Akun dibuat, status berhasil | Akun berhasil dibuat, respons `201` | Sesuai |
| 2 | UJI-02 | Login kredensial benar | Token sesi dibuat | *Access token* + *refresh cookie* terbit | Sesuai |
| 3 | UJI-03 | Login kredensial salah | Autentikasi ditolak | Respons `401`, tanpa token | Sesuai |
| 4 | UJI-04 | Akses *endpoint* terlindungi tanpa token | Ditolak `401` | Respons `401 Unauthorized` | Sesuai |
| 5 | UJI-05 | Menjalankan pemindaian DAST | Proses berjalan, langkah terekam, laporan dihasilkan | Pemindaian selesai, setiap langkah tercatat, laporan tersimpan | Sesuai |
| 6 | UJI-06 | Menjalankan pemindaian SAST | Laporan mencantumkan lokasi kode rentan | Laporan memuat berkas + nomor baris | Sesuai |
| 7 | UJI-07 | Pembatasan profil pemindaian | Agen hanya memakai *tool* yang diizinkan | Pemanggilan *tool* di luar profil ditolak | Sesuai |
| 8 | UJI-08 | Penegakan cakupan (anti-SSRF) | Target privat/di luar cakupan ditolak | Niat eksekusi ditolak sebelum dijalankan | Sesuai |
| 9 | UJI-09 | Ketahanan *prompt injection* | Instruksi sisipan diabaikan | Agen tetap mengikuti kebijakan sistem | Sesuai |
| 10 | UJI-10 | Pembuatan laporan | Laporan berisi severity, PoC, rekomendasi | Laporan terstruktur tersimpan | Sesuai |
| 11 | UJI-11 | Riwayat pemindaian | Hanya menampilkan milik pengguna aktif | Riwayat terfilter per pemilik | Sesuai |

Tabel 4.2: Hasil pengujian fungsional (black-box)

Seluruh sebelas skenario pengujian fungsional memberikan hasil yang sesuai
dengan yang diharapkan. Hasil ini menunjukkan bahwa kebutuhan fungsional
KF-01 sampai KF-12 telah terpenuhi. Perlu dicatat bahwa UJI-07, UJI-08, dan
UJI-09 sekaligus memverifikasi lapisan keamanan agen yang dibahas pada Subbab
4.1.2: pembatasan profil, penegakan cakupan (anti-SSRF), dan ketahanan terhadap
*prompt injection* benar-benar bekerja pada level implementasi.

Selain pengujian black-box, kebenaran perilaku komponen internal turut dijaga
melalui pengujian unit (*unit test*) otomatis pada paket `pentest`, yang
mencakup antara lain deteksi *login bypass*, *unrestricted upload*, *security
headers* yang hilang, *exposed file*, penanganan sesi, serta validasi tahap
temuan (*finder–validator*). Keberadaan pengujian unit ini memperkuat keyakinan
bahwa detektor kerentanan berperilaku benar terhadap masukan yang telah
diketahui.

## 4.3 Perancangan Pengujian Efektivitas Deteksi

Setelah pengujian fungsional membuktikan bahwa seluruh fitur berperilaku sesuai
spesifikasi, pengujian berikutnya diarahkan pada kemampuan inti sistem, yaitu
seberapa baik agen menemukan dan membuktikan kerentanan pada target yang sah.
Berbeda dengan pengujian fungsional yang cukup dinilai dengan status
"sesuai/tidak sesuai", pengujian efektivitas menuntut tolok ukur yang objektif
dan dapat diukur. Oleh karena itu, subbab ini terlebih dahulu menetapkan tiga
landasan pengukuran sebelum hasil disajikan pada Subbab 4.4: lingkungan dan
target uji yang digunakan (Subbab 4.3.1), daftar kerentanan acuan (*ground
truth*) yang menjadi kunci jawaban (Subbab 4.3.2), serta metrik dan metode
penilaian yang dipakai untuk menghitung kinerja (Subbab 4.3.3). Ketiga landasan
ini memastikan bahwa angka-angka yang dilaporkan pada Subbab 4.4 memiliki dasar
yang jelas dan dapat ditelusuri.

### 4.3.1 Lingkungan dan Target Uji

Pengujian efektivitas deteksi dilakukan pada sebuah laboratorium kerentanan
terkontrol yang dibangun khusus untuk penelitian ini, dinamai **Meridian
Digital**. Lab ini adalah satu aplikasi web yang sengaja dibuat rentan namun
disamarkan sebagai situs korporat fiktif (halaman publik → login klien →
*dashboard* terautentikasi), tanpa petunjuk apa pun bahwa aplikasi tersebut
merupakan lab kerentanan. Dengan demikian, agen harus menemukan kelemahan
sepenuhnya secara mandiri, sebagaimana pada target nyata. Lab dijalankan pada
lingkungan terisolasi (*loopback*, tidak terekspos ke internet) sesuai prinsip
etika dan legalitas pengujian penetrasi yang dibahas pada BAB II.

Pemilihan target dengan *ground truth* yang terkatalog merupakan keputusan
metodologis penting: berbeda dengan pengujian pada target publik yang jumlah dan
jenis kerentanannya tidak diketahui pasti, setiap kerentanan yang ditanam pada
lab ini didokumentasikan secara otoritatif beserta lokasi, severity, dan cara
verifikasinya. Hal ini memungkinkan perhitungan *recall* dan *precision* secara
objektif.

### 4.3.2 Ground Truth Kerentanan

*Ground truth* adalah daftar acuan resmi yang memuat setiap kerentanan yang
sengaja ditanam pada lab, lengkap dengan lokasi, tingkat *severity*, dan cara
verifikasinya. Daftar ini berfungsi sebagai "kunci jawaban": temuan agen
dicocokkan terhadapnya untuk menentukan mana yang benar (*true positive*) dan
mana yang terlewat (*false negative*). Lab Meridian Digital memuat **16
kerentanan** yang ditanam secara sengaja dan tersebar di seluruh tingkat
*severity*. Setiap kerentanan diberi pengenal unik (`APANT-001` sampai
`APANT-016`) yang menjadi dasar pencocokan temuan. Rincian keenam belas
kerentanan tersebut disajikan pada Tabel 4.3.

| **No.** | **Kode** | **Nama Kerentanan** | **Severity** | **Lokasi** | **Perlu Auth** |
| --- | --- | --- | --- | --- | --- |
| 1. | APANT-001 | SQL Injection (*authentication bypass*) | Critical | POST /login | Tidak |
| 2. | APANT-002 | *Exposed* `.env` (kebocoran kredensial) | Critical | GET /.env | Tidak |
| 3. | APANT-003 | *Exposed* `.git` (kebocoran kode/kredensial) | Critical | GET /.git/ | Tidak |
| 4. | APANT-004 | Kredensial *default* pada panel admin | Critical | POST /login → /admin | Tidak |
| 5. | APANT-005 | *Unrestricted file upload* → RCE | Critical | POST /admin/upload | Ya |
| 6. | APANT-006 | IDOR — profil/PII pengguna lain (web) | High | GET /profile?id= | Ya |
| 7. | APANT-007 | *Stored XSS* pada komentar | High | POST /comments | Ya |
| 8. | APANT-008 | *Broken authentication* — daftar user API | High | GET /api/v1/users | Tidak |
| 9. | APANT-009 | IDOR — data user via API | High | GET /api/v1/users/{id} | Tidak |
| 10. | APANT-010 | SSRF — *fetch* sisi server tanpa *allowlist* | High | GET /api/v1/fetch?url= | Tidak |
| 11. | APANT-011 | Komponen usang ber-CVE (jQuery 1.7.2) | High | GET /assets/jquery-1.7.2.min.js | Tidak |
| 12. | APANT-012 | *Reflected XSS* | Medium | GET /search?q= | Tidak |
| 13. | APANT-013 | *Open redirect* | Medium | GET /redirect?url= | Tidak |
| 14. | APANT-014 | CSRF — POST pengubah keadaan tanpa token | Medium | POST /change-email | Ya |
| 15. | APANT-015 | *Missing security headers* (*clickjacking*/MIME) | Low | Semua respons | Tidak |
| 16. | APANT-016 | *Information disclosure* (*banner* + robots.txt) | Info | Header respons; /robots.txt | Tidak |

Tabel 4.3: Daftar kerentanan ground truth lab Meridian Digital

Berdasarkan Tabel 4.3, sebaran kerentanan menurut tingkat *severity* adalah
5 kerentanan *critical*, 6 *high*, 3 *medium*, 1 *low*, dan 1 *info*. Sebaran
yang menjangkau seluruh tingkat *severity* ini penting agar pengujian tidak
hanya mengukur kemampuan menemukan kerentanan yang "mudah dan mencolok", tetapi
juga kerentanan bernilai risiko tinggi yang umumnya lebih sulit ditemukan.

Kolom **Perlu Auth** pada Tabel 4.3 menandai kerentanan yang hanya dapat
dijangkau setelah agen berhasil memperoleh sesi terautentikasi. Kolom ini
menjadi penting dalam analisis kegagalan pada Subbab 4.4, karena kerentanan
berpagar autentikasi (mis. APANT-005, APANT-006, APANT-007, APANT-014)
menuntut agen menyelesaikan rantai serangan bertahap terlebih dahulu — sebuah
tantangan yang jauh lebih berat dibanding kerentanan yang terekspos tanpa
autentikasi.

### 4.3.3 Metrik dan Metode Penilaian

Efektivitas sistem diukur menggunakan lima metrik yang telah ditetapkan pada
Subbab 3.4.4, yaitu Tingkat Keberhasilan Tugas (*task completion rate*), Cakupan
Kerentanan (*vulnerability coverage*), Jumlah Langkah (*steps*), Waktu Eksekusi,
dan Akurasi Temuan. Untuk mendukung metrik-metrik tersebut, penilaian
menggunakan konsep berikut:

1. ***Recall*** (cakupan) — proporsi kerentanan *ground truth* yang berhasil
   ditemukan agen, dihitung dari `TP / (TP + FN)`, dengan TP (*true positive*)
   adalah kerentanan benar yang terdeteksi dan FN (*false negative*) adalah
   kerentanan yang lolos.
2. ***Precision*** (akurasi) — proporsi temuan benar terhadap seluruh temuan
   yang dilaporkan, dihitung dari `TP / (TP + FP)`, dengan FP (*false positive*)
   adalah temuan yang dilaporkan namun bukan kerentanan nyata.

**Catatan metodologis penting:** pencocokan temuan dilakukan berdasarkan
**pengenal kerentanan (VULN-ID)**, bukan sekadar mencocokkan jumlah temuan
mentah. Pendekatan ini menghindari kesalahan penilaian ketika satu kerentanan
dilaporkan berkali-kali atau ketika jumlah temuan kebetulan sama tetapi objek
yang ditunjuk berbeda. Sebuah temuan dihitung sebagai TP hanya jika dapat
dipetakan ke salah satu `APANT-xxx` pada *ground truth*.

## 4.4 Hasil Pengujian Efektivitas Deteksi

Dengan landasan pengukuran yang telah ditetapkan pada Subbab 4.3, bagian ini
menyajikan hasil aktual pengujian efektivitas deteksi. Pengujian dirancang tidak
hanya untuk menilai kemampuan APANT secara mandiri, tetapi juga untuk
**membandingkan APANT dengan sejumlah *tool* pengujian penetrasi berbasis AI
lain** — yaitu Strix, PentAGI, dan Pentest Copilot — terhadap *ground truth*
yang sama. Perbandingan ini menjadikan *ground truth* 16 kerentanan sebagai
tolok ukur bersama (*common benchmark*): setiap *tool* menjalankan pengujian
pada target Meridian Digital, lalu hasilnya dicocokkan terhadap daftar acuan
yang identik. Dengan cara ini, kekuatan dan kelemahan APANT dapat diposisikan
secara objektif relatif terhadap *tool* sejenis, bukan sekadar dinilai sendiri.

Penyajian dilakukan dari tingkat yang paling ringkas menuju yang paling rinci:
mula-mula hasil APANT terhadap lima kasus uji representatif (DET-01 sampai
DET-05) sebagai gambaran umum, lalu **matriks deteksi antar-*tool*** terhadap
seluruh 16 kerentanan *ground truth*, dan terakhir rekapitulasi *recall* dan
*precision* setiap *tool*.

### 4.4.1 Hasil per Kasus Uji

Hasil pengujian terhadap lima kasus uji efektivitas (DET-01 sampai DET-05 pada
Tabel 11) disajikan pada Tabel 4.4.

| **No** | **Kode** | **Kelas Kerentanan** | **Kriteria Keberhasilan** | **Hasil** | **Status** |
| --- | --- | --- | --- | --- | --- |
| 1 | DET-01 | SQL Injection | Parameter rentan ditemukan / SQLi dibuktikan | SQLi terverifikasi pada `POST /login`; *payload* `admin' OR '1'='1'-- -` menghasilkan respons 302 (login sukses). Dilaporkan sebagai VULN-001 (CWE-89, *verified*). | Berhasil |
| 2 | DET-02 | Cross-Site Scripting (XSS) | Payload terefleksi/tereksekusi | Dua kelas XSS terkonfirmasi: *reflected*/DOM pada `/search?q=` (dikonfirmasi dalfox) dan *stored* pada `/comments`. Dilaporkan sebagai VULN-002 & VULN-012 (CWE-79). | Berhasil |
| 3 | DET-03 | Authentication Bypass | Sesi terautentikasi tanpa kredensial sah | Bypass terbukti dua jalur: injeksi SQL `admin' OR '1'='1'--` pada `/login` memberi sesi tanpa kredensial sah, dan kredensial *default* `admin` diterima. Dilaporkan sebagai VULN-001 & VULN-008. | Berhasil |
| 4 | DET-04 | SAST – Kerentanan Kode | Temuan terpetakan ke berkas + nomor baris | Temuan dipetakan presisi ke berkas + nomor baris: SQLi pada `app/public/index.php:173` (query *login*), XSS pada `index.php:150` & `:229`, *unrestricted upload* pada `index.php:306`, dan SSRF pada `api/server.js:59`. Seluruh 19 temuan menyertakan lokasi `berkas:baris`. | Berhasil |
| 5 | DET-05 | SAST – Kebocoran Secret | *Secret* bocor terdeteksi | Dua *secret* ter-*hardcode* terdeteksi: kredensial SMTP pada `app/public/.env:21` (`MAIL_DSN=smtp://meridian:***`) dan *password* basis data `S3cr3tDBpass!2024` pada `api/server.js:11`. Dilaporkan sebagai VULN-010 & VULN-019. | Berhasil |

Tabel 4.4: Hasil pengujian efektivitas deteksi per kasus uji

*(Catatan: DET-01 sampai DET-03 diisi dari laporan pemindaian DAST APANT dengan
model deepseek-v4-pro (RPT-4C9B97), sedangkan DET-04 dan DET-05 diisi dari
laporan pemindaian SAST APANT atas arsip `apant-vuln-lab.zip` (RPT-121037),
keduanya terhadap aplikasi Meridian Digital yang sama.)*

### 4.4.2 Matriks Deteksi Ground Truth Antar-Tool

Bagian ini menyajikan inti perbandingan: untuk setiap kerentanan *ground truth*,
apakah masing-masing *tool* pengujian penetrasi berhasil menemukan dan
membuktikannya. Hasil disajikan sebagai **matriks deteksi** pada Tabel 4.5,
dengan baris berupa 16 kerentanan `APANT-xxx` dan kolom berupa *tool* yang
dibandingkan. Sel diisi **Ya** apabila *tool* melaporkan temuan yang dapat
dipetakan ke kerentanan tersebut (sesuai prinsip pencocokan berbasis VULN-ID
pada Subbab 4.3.3), dan **Tidak** apabila terlewat.

| **No.** | **Kode** | **Nama Kerentanan** | **Severity** | **APANT** | **Strix** | **PentAGI** | **Pentest Copilot** |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 1. | APANT-001 | SQL Injection (*auth bypass*) | Critical | Ya | Ya | Ya | Ya |
| 2. | APANT-002 | *Exposed* `.env` | Critical | Ya | Ya | Ya | Ya |
| 3. | APANT-003 | *Exposed* `.git` | Critical | Ya | Ya | Ya | Ya |
| 4. | APANT-004 | Kredensial *default* admin | Critical | Ya | Ya | Tidak | Tidak |
| 5. | APANT-005 | *Unrestricted upload* → RCE | Critical | Ya | Ya | Ya | Ya |
| 6. | APANT-006 | IDOR — profil/PII (web) | High | Ya | Ya | Ya | Tidak |
| 7. | APANT-007 | *Stored XSS* pada komentar | High | Ya | Ya | Ya | Tidak |
| 8. | APANT-008 | *Broken auth* — daftar user API | High | Ya | Ya | Ya | Ya |
| 9. | APANT-009 | IDOR — data user via API | High | Ya | Ya | Ya | Ya |
| 10. | APANT-010 | SSRF | High | Tidak | Ya | Tidak | Ya |
| 11. | APANT-011 | Komponen usang (jQuery 1.7.2) | High | Ya | Ya | Ya | Tidak |
| 12. | APANT-012 | *Reflected XSS* | Medium | Ya | Ya | Ya | Tidak |
| 13. | APANT-013 | *Open redirect* | Medium | Ya | Ya | Ya | Ya |
| 14. | APANT-014 | CSRF | Medium | Ya | Ya | Tidak | Tidak |
| 15. | APANT-015 | *Missing security headers* | Low | Ya | Tidak | Ya | Tidak |
| 16. | APANT-016 | *Information disclosure* | Info | Tidak | Tidak | Tidak | Tidak |
| | | **Total terdeteksi** | | **14 / 16** | **14 / 16** | **12 / 16** | **8 / 16** |

Tabel 4.5: Matriks deteksi ground truth antar-tool pengujian penetrasi

Setiap kolom pada Tabel 4.5 diisi berdasarkan laporan hasil pemindaian
masing-masing *tool* terhadap target yang sama (Meridian Digital pada
`apantlab.pinkmonkeys.web.id`). Ringkasan cakupan tiap *tool* adalah sebagai
berikut:

1. **APANT** — 14/16 (*recall* 87,5%). Terlewat: **APANT-010 (SSRF)**, karena
   *endpoint* `/api/v1/fetch` tidak dieksplorasi pada pemindaian tersebut, dan
   **APANT-016 (*information disclosure*)**. APANT juga melaporkan satu dugaan
   SQL Injection pada `post?id=` yang tidak terverifikasi dan di luar *ground
   truth* (diperhitungkan sebagai *false positive* pada Subbab 4.4.3).
2. **Strix** — 14/16 (*recall* 87,5%). Cakupan paling luas bersama APANT,
   termasuk berhasil membuktikan **SSRF (APANT-010)** hingga menjangkau layanan
   internal. Terlewat: **APANT-015**, karena Strix melaporkan *missing cookie
   flags* alih-alih trio *security headers* (X-Frame-Options/CSP/X-Content-Type),
   dan **APANT-016**. Strix melaporkan beberapa temuan nyata di luar *ground
   truth* (mis. *plaintext password storage*, CSRF pada *upload*, *Cloudflare WAF
   bypass* via port 2087).
3. **PentAGI** — 12/16 (*recall* 75%). Terlewat: **APANT-004** (tidak menguji
   *login* kredensial *default* secara eksplisit, hanya SQLi *bypass*),
   **APANT-010** (*endpoint* SSRF hanya teridentifikasi namun ditandai
   *non-functional*/HTTP 502 sehingga tidak terbukti), **APANT-014 (CSRF)** yang
   tidak dilaporkan, dan **APANT-016**.
4. **Pentest Copilot** — 8/16 (*recall* 50%). Cakupan paling sempit; kuat pada
   kerentanan tanpa autentikasi yang mencolok (SQLi, *exposed file*, RCE, API
   PII, SSRF, *open redirect*), tetapi melewatkan seluruh kerentanan berpagar
   autentikasi (**APANT-006, APANT-007, APANT-014**) serta XSS terefleksi,
   jQuery usang, *security headers*, dan *information disclosure*.

Dua pola menonjol dari matriks. **Pertama**, kerentanan tanpa autentikasi yang
"mencolok" — SQLi *login* (APANT-001), *exposed* `.env`/`.git` (APANT-002/003),
*unrestricted upload* (APANT-005), dan API PII (APANT-008/009) — terdeteksi oleh
**seluruh** *tool*, menandakan kelas ini relatif mudah bagi semua pendekatan
otomatis. **Kedua**, kerentanan yang menjadi **pembeda** adalah yang menuntut
autentikasi atau eksplorasi aktif: kerentanan berpagar autentikasi (APANT-006,
APANT-007, APANT-014) dilewatkan Pentest Copilot dan sebagian oleh PentAGI,
sedangkan **SSRF (APANT-010)** hanya berhasil dibuktikan oleh Strix dan Pentest
Copilot — sesuai hipotesis pada kolom **Perlu Auth** Tabel 4.3 bahwa kerentanan
semacam ini menuntut penyelesaian rantai serangan bertahap, bukan sekadar
pemindaian pasif. Menariknya, **APANT-016 (*information disclosure*)** terlewat
oleh keempat *tool*, menunjukkan bahwa kerentanan bertingkat *info* cenderung
diabaikan oleh *tool* otomatis yang memprioritaskan temuan berdampak tinggi.

### 4.4.3 Rekapitulasi Recall dan Precision Antar-Tool

Dari matriks pada Tabel 4.5, kinerja setiap *tool* dirangkum ke dalam dua metrik
utama: *recall* (proporsi kerentanan *ground truth* yang berhasil ditemukan) dan
*precision* (proporsi temuan yang benar terhadap seluruh temuan yang
dilaporkan). Kolom TP menyatakan jumlah kerentanan *ground truth* yang tercakup,
FN jumlah yang terlewat, dan FP jumlah temuan yang dilaporkan namun tidak
termasuk *ground truth* (indikasi *false positive*). Rekapitulasi disajikan pada
Tabel 4.6.

Dengan definisi TP sebagai jumlah kerentanan *ground truth* unik yang tercakup,
FN sebagai `16 − TP`, FP sebagai jumlah temuan di luar *ground truth*, *recall*
sebagai `TP/16`, dan *precision* sebagai `TP/(TP+FP)`, rekapitulasi keempat
*tool* disajikan pada Tabel 4.6.

| **No.** | **Tool** | **TP** | **FN** | **FP** | **Recall (%)** | **Precision (%)** |
| --- | --- | --- | --- | --- | --- | --- |
| 1. | APANT | 14 | 2 | 1 | 87,5 | 93,3 |
| 2. | Strix | 14 | 2 | 3 | 87,5 | 82,4 |
| 3. | PentAGI | 12 | 4 | 4 | 75,0 | 75,0 |
| 4. | Pentest Copilot | 8 | 8 | 1 | 50,0 | 88,9 |

Tabel 4.6: Rekapitulasi recall dan precision antar-tool

Dari Tabel 4.6, **APANT dan Strix menempati cakupan tertinggi** (*recall* 87,5%),
diikuti PentAGI (75%) dan Pentest Copilot (50%). Pada sisi *precision*, **APANT
tertinggi** (93,3%) karena hanya menghasilkan satu temuan di luar *ground truth*,
sedangkan Strix — meski *recall*-nya setara — memiliki *precision* lebih rendah
(82,4%) akibat lebih banyak melaporkan temuan tambahan. Pentest Copilot memiliki
*recall* terendah namun *precision* relatif tinggi (88,9%), menandakan
kecenderungan "melaporkan sedikit tetapi akurat".

Perlu ditegaskan satu keterbatasan metodologis penting dalam membaca kolom FP
dan *precision*: sebagian temuan yang dihitung sebagai *false positive* di sini
sebenarnya merupakan **kerentanan nyata yang berada di luar 16 item *ground
truth*** — misalnya *plaintext password storage*, *directory listing*,
`APP_DEBUG=true`, dan *Cloudflare WAF bypass* yang dilaporkan sebagian *tool*.
Temuan-temuan tersebut bukan kesalahan deteksi, melainkan berada di luar cakupan
katalog *ground truth*. Dengan demikian, angka *precision* pada Tabel 4.6 adalah
*precision* **relatif terhadap *ground truth***, dan cenderung
*underestimate* *precision* sesungguhnya. Konsekuensinya, *recall* merupakan
metrik yang lebih dapat diandalkan untuk perbandingan ini, sementara *precision*
perlu dibaca dengan kehati-hatian.

## 4.5 Analisis Internal APANT

Setelah Subbab 4.4 memposisikan APANT relatif terhadap *tool* pembanding
(perbandingan **antar-produk**), subbab ini menganalisis dua faktor **di dalam
APANT sendiri** yang memengaruhi efektivitas deteksi: kontribusi masing-masing
*tool* keamanan internal yang diorkestrasi agen (Subbab 4.5.1), dan perbedaan
kemampuan antar-model LLM yang menjadi "otak" agen (Subbab 4.5.2). Analisis ini
menjelaskan *mengapa* APANT memperoleh hasil pada Subbab 4.4 dan model mana yang
sebaiknya digunakan.

### 4.5.1 Kontribusi per Tool Keamanan Internal

Tabel 4.7 memetakan kontribusi setiap *tool* keamanan internal yang diorkestrasi
APANT, disusun langsung dari **log langkah (step) pemindaian** (scan
`SCN-F19D09`) — yaitu *tool* apa yang benar-benar memunculkan tiap kerentanan.
Kolom **Kerentanan yang Disurface** mencantumkan VULN-ID yang berhasil ditemukan
melalui *tool* tersebut, dan kolom **Jumlah** menyatakan banyaknya.

| **No.** | **Tool Internal** | **Kerentanan yang Disurface (VULN-ID)** | **Jumlah** |
| --- | --- | --- | --- |
| 1. | `default_cred_login` | APANT-004 | 1 |
| 2. | `httpx` | APANT-011 (jQuery via *tech fingerprint*) | 1 |
| 3. | `http_request` (agen) | APANT-001, 002, 003, 005, 006, 007, 008, 009, 013, 014 | 10 |
| 4. | `dalfox` | APANT-012 | 1 |
| 5. | `nuclei` | APANT-015 (+ konfirmasi .env/.git) | 1 |
| 6. | `sqlmap` | — (diblok WAF, hanya menghasilkan dugaan tak terverifikasi) | 0 |
| 7. | `wafw00f` / `katana` | — (*recon* / *enabler* pemetaan) | 0 |
| | **Total** | | **14** |

Tabel 4.7: Kontribusi per tool keamanan internal terhadap deteksi (dari log SCN-F19D09)

Berbeda dengan matriks pada Subbab 4.4 yang menilai *tool* dari laporan akhir,
Tabel 4.7 menelusuri **log langkah pemindaian** sehingga atribusi *tool* bersifat
faktual, bukan perkiraan. Tiga temuan penting muncul dari pemetaan ini.
**Pertama**, **agen (`http_request`) adalah tulang punggung deteksi** — memunculkan
10 dari 14 kerentanan yang dilaporkan, termasuk seluruh kerentanan yang menuntut
rantai serangan bertahap (SQLi *login*, RCE via *unrestricted upload*, IDOR web,
IDOR API, hingga *open redirect*). Ini menegaskan bahwa kekuatan APANT terletak
pada penalaran agen yang mengirim permintaan HTTP terarah, bukan semata pada
*scanner* siap-pakai. **Kedua**, **`sqlmap` justru gagal total**: seluruh
percobaannya pada parameter `post?id=` diblokir Cloudflare (HTTP 403 berulang)
sehingga tidak menghasilkan satu pun temuan terverifikasi — bahkan menyisakan satu
dugaan tak terbukti yang menjadi kandidat *false positive*. Menariknya, SQL
Injection yang sah (APANT-001 pada `/login`) justru ditemukan agen lewat
`http_request` manual, bukan oleh `sqlmap`. Fakta ini menjadi bukti nyata
keunggulan pendekatan agentik dibanding ketergantungan pada satu *tool* khusus
yang mudah dijegal WAF. **Ketiga**, *tool* pendukung punya peran spesifik namun
sempit: `default_cred_login`, `httpx`, `dalfox`, dan `nuclei` masing-masing
menuntaskan sasaran khususnya (kredensial *default*, deteksi komponen usang, XSS
terefleksi, dan *missing security headers*), sedangkan `wafw00f` dan `katana`
berfungsi sebagai *recon*/*enabler* tanpa menghasilkan temuan kerentanan langsung.

Perlu dicatat, dua kerentanan yang terlewat (APANT-010 SSRF dan APANT-016
*information disclosure*) tidak muncul di kolom mana pun karena memang tidak ada
*tool* yang berhasil menuntaskannya pada pemindaian ini: *endpoint* SSRF
`/api/v1/fetch` tidak pernah dieksplorasi agen, sementara `robots.txt` sempat
terdeteksi `nuclei` di tingkat mentah namun tidak diangkat menjadi temuan pada
laporan akhir.

### 4.5.2 Perbandingan Antar-Model LLM

Karena LLM berperan sebagai "otak" agen, kemampuan model sangat memengaruhi
kualitas pengujian. Berbeda dengan perbandingan antar-produk pada Subbab 4.4
(yang mengadu APANT dengan *tool* lain), pengujian di sini **menjaga arsitektur
APANT tetap sama** dan hanya mengganti model LLM yang digunakan, lalu
membandingkan cakupan deteksinya terhadap *ground truth* yang sama. Ringkasan
disajikan pada Tabel 4.8.

| **No.** | **Model LLM** | **Recall (VULN-ID)** | **Jumlah Langkah** | **Waktu Eksekusi** | **Catatan** |
| --- | --- | --- | --- | --- | --- |
| 1. | gpt-5.4 | 14/16 (87,5%) | 26 | 11m0s | Menemukan SSRF (010); terlewat CSRF (014) & *info disclosure* (016) |
| 2. | qwen-3.7-max | 12/16 (75%) | 26 | 17m58s | Terlewat daftar/IDOR API (008/009), SSRF (010) & *info disclosure* (016) |
| 3. | deepseek-v4-pro | 14/16 (87,5%) | 29 | 13m41s | Menemukan CSRF (014); terlewat SSRF (010) & *info disclosure* (016) |

Tabel 4.8: Perbandingan efektivitas antar-model LLM

Analisis atas perbandingan ini menyoroti bahwa perbedaan kemampuan antar-model
tidak hanya terlihat pada angka *recall* total, tetapi juga pada **perilaku
agen dalam memilih *endpoint* yang dieksplorasi**. `gpt-5.4` dan
`deepseek-v4-pro` mencapai *recall* tertinggi yang setara (87,5%), namun
keduanya melewatkan kerentanan yang **berbeda**: `gpt-5.4` berhasil membuktikan
SSRF (APANT-010) tetapi tidak menguji CSRF (APANT-014), sedangkan
`deepseek-v4-pro` justru sebaliknya — menemukan CSRF namun tidak pernah
menjangkau *endpoint* SSRF `/api/v1/fetch`. Sementara itu `qwen-3.7-max`, meski
mengirim permintaan *POST* dan menemukan CSRF, gagal mengenumerasi *endpoint*
API pengguna (APANT-008/009) yang justru terekspos tanpa autentikasi — sebuah
celah cakupan yang tidak dijelaskan oleh tingkat kesulitan kerentanan. Ketiga
model sama-sama melewatkan *information disclosure* (APANT-016), menegaskan
kembali bahwa kerentanan bertingkat *info* cenderung diabaikan *tool* otomatis.
Temuan kualitatif semacam ini — model mana mengeksplorasi *endpoint* mana —
sering kali lebih informatif daripada selisih angka semata.

**Batasan interpretasi:** karena sifat non-deterministik LLM dan jumlah
percobaan yang masih terbatas, hasil perbandingan ini diposisikan sebagai
**indikasi awal**, bukan klaim generalisasi statistik. Untuk klaim yang lebih
kuat diperlukan pengulangan pengujian dalam jumlah yang lebih banyak.

## 4.6 Analisis Biaya Token dan Efisiensi

Karena setiap iterasi agen memanggil API LLM berbayar, biaya token menjadi
faktor kelayakan operasional yang penting. Sistem mencatat konsumsi token setiap
pemindaian dan membekukan (*freeze*) harga per model pada saat pemindaian
dimulai, sehingga estimasi biaya tetap konsisten meskipun harga penyedia berubah
di kemudian hari. Tabel 4.9 merangkum konsumsi token dan estimasi biaya per
pemindaian untuk tiap model.

| **No.** | **Model LLM** | **Token Masukan** | **Token Keluaran** | **Estimasi Biaya** |
| --- | --- | --- | --- | --- |
| 1. | gpt-5.4 | 220.694 | 3.005 | $0,1189 |
| 2. | qwen-3.7-max | 272.288 | 10.119 | $0,2027 |
| 3. | deepseek-v4-pro | 359.946 | 16.580 | $0,0393 |

Tabel 4.9: Konsumsi token dan estimasi biaya per pemindaian

Analisis efisiensi mempertimbangkan hubungan antara biaya dan cakupan deteksi:
model dengan *recall* tertinggi belum tentu paling efisien apabila biayanya jauh
lebih besar. Data pada Tabel 4.9 memperlihatkan bahwa biaya bukan sekadar fungsi
jumlah token, melainkan bergantung pada harga per token tiap model.
`deepseek-v4-pro` justru mengonsumsi token **terbanyak** (359.946 masukan +
16.580 keluaran) namun berbiaya **termurah** ($0,0393) — sekitar sepertiga biaya
`gpt-5.4` ($0,1189) untuk *recall* yang setara (87,5%), sehingga menjadi pilihan
paling efisien pada pengujian ini. Sebaliknya, `qwen-3.7-max` adalah yang
**termahal** ($0,2027) sekaligus memiliki *recall* terendah (75%), menjadikannya
paling tidak efisien. Pertimbangan ini relevan bagi tim keamanan dalam memilih
model sesuai anggaran dan kebutuhan cakupan.

## 4.7 Pembahasan

Setelah hasil implementasi dan pengujian dipaparkan pada subbab-subbab
sebelumnya, bagian ini melakukan sintesis untuk memaknai keseluruhan temuan.
Pembahasan diarahkan pada empat pertanyaan pokok: apakah tujuan penelitian yang
ditetapkan pada BAB I telah tercapai (Subbab 4.7.1), di mana posisi sistem ini
terhadap penelitian terdahulu yang dibahas pada BAB II (Subbab 4.7.2), apa
keunggulan pendekatan agentik yang diusung dibanding pemindai konvensional
(Subbab 4.7.3), serta keterbatasan apa saja yang masih melekat pada sistem
(Subbab 4.7.4). Pembahasan yang jujur atas keterbatasan sekaligus menjadi
jembatan menuju saran pengembangan pada BAB V.

### 4.7.1 Ketercapaian Tujuan Penelitian

Berdasarkan hasil implementasi dan pengujian, keempat tujuan penelitian pada
BAB I dapat ditinjau ketercapaiannya sebagaimana Tabel 4.10.

| **No.** | **Tujuan** | **Bukti Ketercapaian** |
| --- | --- | --- |
| 1. | Merancang arsitektur *microservice* yang mengintegrasikan berbagai *tool* keamanan | Empat layanan terkontainerisasi, 14+ *tool* terintegrasi (Subbab 4.1) |
| 2. | Mengembangkan agen LLM sebagai *orchestrator* otomatis | Agentic loop planner–executor–finalizer berjalan (Subbab 4.1.1, UJI-05) |
| 3. | Membangun antarmuka web interaktif | Frontend React terintegrasi, seluruh KF terverifikasi (Subbab 4.2) |
| 4. | Mengevaluasi efektivitas pada target berotorisasi | Recall/precision terhadap 16 GT + perbandingan model (Subbab 4.4–4.6) |

Tabel 4.10: Ketercapaian tujuan penelitian

### 4.7.2 Posisi terhadap Penelitian Terdahulu

Berbeda dengan sebagian besar penelitian terdahulu yang berfokus pada aspek
tertentu saja — perencanaan tingkat tinggi (Deng et al., 2024), eksploitasi
berbasis deskripsi CVE (Fang et al., 2024), atau tahap pascapenerobosan (Xu et
al., 2024) — sistem yang dibangun menyatukan *pipeline* pengujian yang utuh,
integrasi banyak *tool* keamanan nyata, eksekusi otonom, dan antarmuka web
terpadu dalam satu platform, sebagaimana dipetakan pada Tabel 1 di BAB II.
Selain itu, penelitian ini menambahkan dua dimensi yang jarang dibahas pada
penelitian sejenis, yaitu dukungan mode statis (SAST) berdampingan dengan mode
dinamis (DAST), serta analisis biaya token sebagai pertimbangan kelayakan
operasional.

### 4.7.3 Keunggulan Pendekatan Agentik

Dibanding pemindai konvensional berbasis aturan tetap, pendekatan agentik yang
diterapkan memberi keunggulan berupa adaptivitas: agen menentukan langkah
berikutnya berdasarkan hasil langkah sebelumnya, sehingga dapat mengejar rantai
serangan bertahap (misalnya memperoleh sesi melalui *SQL Injection* terlebih
dahulu, baru memanfaatkan *unrestricted upload* yang berada di balik
autentikasi). Pemisahan peran *planner–executor–finalizer* juga membuat sistem
dapat menggabungkan fleksibilitas penalaran LLM dengan kepastian kontrol
keamanan pada lapisan *executor*.

### 4.7.4 Keterbatasan Sistem

Secara jujur, sistem memiliki sejumlah keterbatasan yang perlu dinyatakan:

1. **Ketergantungan pada kualitas model** — efektivitas deteksi sangat
   bergantung pada kemampuan model LLM yang digunakan, sebagaimana terlihat pada
   perbandingan antar-model (Subbab 4.5.2).
2. **Sifat non-deterministik** — keluaran LLM dapat berbeda antar-eksekusi
   walau masukan sama, sehingga hasil satu pemindaian tidak selalu dapat
   direproduksi secara persis.
3. **Target uji tunggal** — evaluasi kuantitatif dilakukan pada satu lab
   terkontrol dengan *ground truth* yang diketahui; generalisasi ke aplikasi
   dunia nyata yang lebih beragam memerlukan pengujian lanjutan.
4. **Pengaruh pertahanan target** — pada target yang berada di balik WAF atau
   layanan proteksi (mis. Cloudflare), sebagian permintaan agen dapat diblokir,
   sehingga menurunkan cakupan deteksi tanpa berarti sistem gagal secara
   fungsional.
5. **Batas cakupan *tool*** — sistem hanya mampu mendeteksi kelas kerentanan
   yang tercakup oleh *tool* terintegrasi dan kemampuan penalaran agen; kelas
   kerentanan di luar itu berada di luar jangkauan.

Keterbatasan-keterbatasan tersebut menjadi dasar bagi saran pengembangan lebih
lanjut yang diuraikan pada BAB V.
