# KESIMPULAN DAN SARAN

## 5.1 Kesimpulan

Berdasarkan hasil perancangan, implementasi, dan pengujian sistem otomasi *web
penetration testing* berbasis kecerdasan buatan (APANT) yang telah diuraikan
pada bab-bab sebelumnya, dapat ditarik kesimpulan sebagai berikut:

1. Arsitektur sistem otomasi pengujian penetrasi web berbasis *microservice*
   berhasil dirancang dan diimplementasikan dengan mengintegrasikan berbagai
   alat keamanan ke dalam satu platform terpadu. Sistem terdiri atas empat
   layanan yang dikemas dalam kontainer dan diorkestrasi menggunakan Docker
   Compose, yaitu *frontend*, *backend* API, *scanner*, dan basis data
   PostgreSQL. Pemisahan layanan *scanner* dari *backend* API menjadi keputusan
   arsitektural yang penting: seluruh *tool* berisiko tinggi (sqlmap, nmap,
   nuclei, dan lainnya) hanya dijalankan di dalam kontainer *scanner* yang
   diterapkan prinsip *least privilege* (mode *read-only*, *no-new-privileges*,
   kapabilitas Linux terbatas, dan pengguna *non-root*), sehingga kompromi pada
   lapisan *tool* tidak langsung membahayakan layanan inti. Hal ini menjawab
   rumusan masalah pertama.

2. Agen AI berbasis *Large Language Model* (LLM) berhasil diimplementasikan
   sebagai *orchestrator* yang mampu memilih, menjalankan, dan menginterpretasi
   hasil berbagai alat keamanan secara otomatis. Agen menerapkan pola ReAct
   (*reasoning–acting*) dengan tiga peran, yaitu *planner* (LLM memutuskan
   tindakan berikutnya dalam format JSON terstruktur), *executor* (*backend*
   memvalidasi dan mengeksekusi niat pemanggilan *tool*), dan *finalizer* (LLM
   menyusun ringkasan penilaian keamanan). Setiap niat pemanggilan *tool*
   melewati empat lapis penjagaan secara berurutan — pembatasan profil,
   penegakan cakupan (anti-SSRF), penjagaan *payload*, dan validasi kebijakan —
   yang sekaligus menjadi pertahanan terhadap *prompt injection* melalui
   penandaan seluruh keluaran *tool* sebagai data tak tepercaya. Sistem juga
   mendukung dua mode pengujian, yaitu dinamis (DAST) untuk aplikasi web yang
   berjalan dan statis (SAST) untuk analisis kode sumber. Hal ini menjawab
   rumusan masalah kedua.

3. Sistem berhasil menghasilkan laporan pengujian penetrasi secara otomatis
   berdasarkan temuan dari berbagai alat yang dijalankan. Laporan yang
   dihasilkan bersifat terstruktur, memuat ringkasan eksekutif, daftar
   kerentanan beserta tingkat *severity*, *Proof of Concept* (PoC), dan
   rekomendasi mitigasi. Khusus mode SAST, laporan mencantumkan lokasi temuan
   secara presisi hingga ke berkas dan nomor baris. Dukungan antarmuka web yang
   interaktif memudahkan pengguna memasukkan target, memilih jenis pemindaian,
   dan meninjau hasil. Hal ini menjawab rumusan masalah ketiga.

4. Berdasarkan pengujian, efektivitas sistem dalam melakukan pengujian penetrasi
   pada target berotorisasi dapat dibuktikan. Seluruh sebelas skenario pengujian
   fungsional (*black-box*, UJI-01 sampai UJI-11) memberikan hasil yang sesuai
   dengan yang diharapkan, sehingga kebutuhan fungsional KF-01 sampai KF-12
   terpenuhi. Pengujian efektivitas deteksi terhadap laboratorium kerentanan
   terkontrol (Meridian Digital) dengan 16 kerentanan *ground truth* yang
   terkatalog menunjukkan bahwa sistem mampu menemukan dan membuktikan
   kerentanan secara otomatis dengan intervensi manusia yang minimal.
   Dibandingkan pendekatan manual konvensional yang menyita waktu dan menuntut
   keahlian intensif, pendekatan agentik ini memberi keunggulan berupa
   adaptivitas (agen menentukan langkah berikutnya berdasarkan hasil langkah
   sebelumnya sehingga dapat mengejar rantai serangan bertahap) dan efisiensi
   waktu. Hal ini menjawab rumusan masalah keempat.

Secara keseluruhan, penelitian ini berhasil membangun sebuah sistem otomasi *web
penetration testing* berbasis AI yang menyatukan *pipeline* pengujian yang utuh,
integrasi banyak *tool* keamanan nyata, eksekusi otonom, serta antarmuka web
terpadu dalam satu platform — sebuah kombinasi yang belum dipenuhi secara
lengkap oleh penelitian terdahulu sebagaimana dipetakan pada BAB II.

## 5.2 Saran

Sistem yang dibangun masih berstatus prototipe dan memiliki sejumlah
keterbatasan sebagaimana diuraikan pada Subbab 4.7.4. Untuk pengembangan lebih
lanjut, disarankan beberapa hal berikut:

1. **Memperluas ragam target uji.** Evaluasi kuantitatif pada penelitian ini
   dilakukan pada satu laboratorium terkontrol dengan *ground truth* yang
   diketahui. Pengujian lanjutan sebaiknya menyertakan lebih banyak target
   dengan variasi teknologi dan kelas kerentanan yang beragam (misalnya
   PortSwigger, Hack The Box, atau aplikasi rentan lain) agar generalisasi
   kemampuan sistem terhadap aplikasi dunia nyata dapat diuji lebih kuat.

2. **Menambah pengulangan pengujian untuk klaim yang lebih kuat.** Karena
   keluaran LLM bersifat non-deterministik, hasil satu kali pemindaian tidak
   selalu dapat direproduksi secara persis. Perbandingan antar-model sebaiknya
   dilakukan dengan jumlah pengulangan yang lebih banyak dan dianalisis secara
   statistik, sehingga hasilnya tidak lagi sekadar indikasi awal melainkan klaim
   yang dapat digeneralisasi.

3. **Meningkatkan ketahanan terhadap pertahanan target.** Pada target yang
   berada di balik WAF atau layanan proteksi (mis. Cloudflare), sebagian
   permintaan agen dapat diblokir sehingga menurunkan cakupan deteksi.
   Pengembangan mekanisme deteksi pemblokiran (*WAF/firewall block detection*)
   dan penyesuaian strategi agen ketika terdeteksi pemblokiran dapat
   meningkatkan keandalan pengujian pada target berpertahanan.

4. **Memperluas cakupan kelas kerentanan dan *tool*.** Sistem saat ini hanya
   mampu mendeteksi kelas kerentanan yang tercakup oleh *tool* terintegrasi dan
   kemampuan penalaran agen. Penambahan *tool* keamanan baru serta perluasan
   profil pemindaian akan memperluas jangkauan deteksi ke kelas kerentanan yang
   belum tercakup.

5. **Menyempurnakan kemampuan agen menembus fitur berpagar autentikasi.**
   Kerentanan yang hanya dapat dijangkau setelah autentikasi (mis. *unrestricted
   upload*, IDOR web, *stored XSS*, dan CSRF) menuntut agen menyelesaikan rantai
   serangan bertahap terlebih dahulu. Penguatan alur *auto-login* dan manajemen
   sesi agen dapat meningkatkan cakupan deteksi pada kerentanan berpagar
   autentikasi.

6. **Menambahkan modul remediasi otomatis.** Sistem saat ini berhenti pada
   pelaporan temuan dan rekomendasi. Pengembangan modul yang menyarankan
   strategi mitigasi optimal secara otomatis — bahkan menghasilkan usulan
   perbaikan kode pada mode SAST — akan meningkatkan nilai guna sistem bagi tim
   pengembang.

7. **Menyediakan umpan balik progres pemindaian secara *real-time*.** Untuk
   meningkatkan aspek ketergunaan, antarmuka web dapat dikembangkan agar
   menampilkan progres pemindaian secara langsung (langkah agen, *tool* yang
   sedang berjalan, dan temuan sementara), sehingga pengguna memperoleh
   transparansi penuh selama proses pengujian berlangsung.

8. **Mengoptimalkan biaya token dan efisiensi.** Karena setiap iterasi agen
   memanggil API LLM berbayar, penelitian lanjutan dapat mengeksplorasi strategi
   penghematan biaya, seperti manajemen konteks yang lebih efisien, pemilihan
   model secara adaptif sesuai kompleksitas target, atau pemanfaatan *caching*,
   tanpa mengorbankan cakupan deteksi.
