# Dokumentasi Tools Pentest - APANT Scanner
> Versi: 1.0 | Tanggal: Mei 2026

---

## Daftar Tools

| No | Tool | Versi | Fungsi Utama |
|----|------|-------|--------------|
| 1 | nmap | 7.95 | Port scanning & service detection |
| 2 | httpx | v1.9.0 | HTTP probing & fingerprinting |
| 3 | subfinder | v2.x | Subdomain enumeration |
| 4 | katana | v1.5.0 | Web crawler |
| 5 | gau | v2.x | URL enumeration dari arsip |
| 6 | waybackurls | v0.1.0 | URL dari Wayback Machine |
| 7 | ffuf | v2.1.0 | Web fuzzing |
| 8 | nuclei | v3.8.0 | Vulnerability scanning |
| 9 | dalfox | v2.12.0 | XSS scanning |
| 10 | sqlmap | v1.10.5 | SQL injection testing |
| 11 | mitmdump | v12.2.2 | HTTP/HTTPS proxy & interception |

---

## 1. nmap

### Deskripsi
Nmap (Network Mapper) adalah tool open-source untuk network discovery dan security auditing. Digunakan untuk mendeteksi host aktif, port terbuka, service yang berjalan, OS, dan informasi jaringan lainnya.

### Lokasi Binary
```
/usr/bin/nmap
```

### Capabilities
Container dikonfigurasi dengan `cap_net_raw` dan `cap_net_admin` serta `setcap cap_net_raw,cap_net_admin+eip` agar nmap bisa melakukan raw packet scanning tanpa root.

### Penggunaan Dasar
```bash
# Scan port dasar
nmap -sV <target>

# Scan dengan deteksi versi service
nmap -sV --version-intensity 5 <target>

# Scan semua port
nmap -p- <target>

# Scan dengan script NSE
nmap --script=default <target>

# OS detection
nmap -O <target>

# Scan UDP
nmap -sU <target>

# Output ke file JSON (via XML)
nmap -oX output.xml <target>

# Aggressive scan
nmap -A <target>
```

### Flag Penting
| Flag | Fungsi |
|------|--------|
| `-sV` | Service version detection |
| `-sS` | SYN scan (stealth) |
| `-sU` | UDP scan |
| `-p-` | Scan semua 65535 port |
| `-O` | OS detection |
| `-A` | Aggressive (OS+version+script+traceroute) |
| `-oX` | Output XML |
| `-oN` | Output normal |
| `--script` | Jalankan NSE script |
| `-T0` s/d `-T5` | Timing (0=paranoid, 5=insane) |

### Contoh Output
```
PORT    STATE SERVICE VERSION
22/tcp  open  ssh     OpenSSH 6.6.1p1
80/tcp  open  http    Apache httpd 2.4.7
443/tcp open  https   Apache httpd 2.4.7
```

---

## 2. httpx

### Deskripsi
httpx adalah HTTP toolkit dari ProjectDiscovery untuk melakukan probing terhadap daftar host secara cepat. Menduksi deteksi teknologi, status code, title, redirect, dan banyak informasi HTTP lainnya.

### Lokasi Binary
```
/usr/local/bin/httpx
```

### Penggunaan Dasar
```bash
# Probe single URL
httpx -u https://target.com

# Probe dengan info lengkap
httpx -u https://target.com -title -status-code -tech-detect -content-length

# Probe dari list file
httpx -l urls.txt -title -status-code

# Probe dengan follow redirect
httpx -u https://target.com -follow-redirects

# Probe dengan screenshot (headless)
httpx -u https://target.com -screenshot

# Output JSON
httpx -u https://target.com -json -o output.json

# Filter status code tertentu
httpx -l urls.txt -mc 200,301,302

# Probe dengan custom header
httpx -u https://target.com -H "Authorization: Bearer token"
```

### Flag Penting
| Flag | Fungsi |
|------|--------|
| `-u` | Single URL target |
| `-l` | List URL dari file |
| `-title` | Ambil HTML title |
| `-status-code` | Tampilkan HTTP status code |
| `-tech-detect` | Deteksi teknologi (Wappalyzer) |
| `-content-length` | Tampilkan ukuran response |
| `-follow-redirects` | Ikuti redirect |
| `-json` | Output JSON |
| `-o` | Output ke file |
| `-mc` | Match status code tertentu |
| `-fc` | Filter status code tertentu |
| `-silent` | Output minimal |
| `-timeout` | Timeout per request (detik) |
| `-threads` | Jumlah thread concurrent |

### Contoh Output
```
https://target.com [200] [Apache/2.4.7] [Login Page] [WordPress]
```

---

## 3. subfinder

### Deskripsi
Subfinder adalah tool passive subdomain enumeration dari ProjectDiscovery. Menggunakan berbagai sumber publik (DNS, certificate transparency, search engines, dll) untuk menemukan subdomain tanpa melakukan brute force.

### Lokasi Binary
```
/usr/local/bin/subfinder
```

### Sumber Data
Subfinder menggunakan 40+ sumber termasuk: Shodan, VirusTotal, SecurityTrails, CertSpotter, crt.sh, HackerTarget, PassiveTotal, dan lainnya.

### Penggunaan Dasar
```bash
# Enumerate subdomain domain tertentu
subfinder -d target.com

# Output silent (hanya subdomain)
subfinder -d target.com -silent

# Output ke file
subfinder -d target.com -o subdomains.txt

# Scan multiple domain
subfinder -dL domains.txt

# Include subdomain dari semua sumber
subfinder -d target.com -all

# Output JSON
subfinder -d target.com -json -o output.json

# Dengan recursive enumeration
subfinder -d target.com -recursive

# Set timeout
subfinder -d target.com -timeout 30
```

### Flag Penting
| Flag | Fungsi |
|------|--------|
| `-d` | Target domain |
| `-dL` | List domain dari file |
| `-o` | Output ke file |
| `-silent` | Hanya tampilkan subdomain |
| `-all` | Gunakan semua sumber |
| `-recursive` | Recursive enumeration |
| `-json` | Output JSON |
| `-timeout` | Timeout per sumber |
| `-t` | Jumlah thread |
| `-nW` | Hilangkan wildcard subdomain |

### Contoh Output
```
mail.target.com
www.target.com
api.target.com
dev.target.com
staging.target.com
```

---

## 4. katana

### Deskripsi
Katana adalah next-generation web crawling framework dari ProjectDiscovery. Mendukung JavaScript rendering, form filling otomatis, dan crawling modern web application.

### Lokasi Binary
```
/usr/local/bin/katana
```

### Penggunaan Dasar
```bash
# Crawl basic
katana -u https://target.com

# Crawl dengan kedalaman tertentu
katana -u https://target.com -depth 3

# Crawl dengan JavaScript rendering (headless)
katana -u https://target.com -headless

# Output ke file
katana -u https://target.com -o urls.txt

# Crawl dari list URL
katana -list urls.txt

# Filter extension tertentu
katana -u https://target.com -extension-filter jpg,png,gif

# Tampilkan hanya URL dengan parameter
katana -u https://target.com -f qurl

# Output JSON
katana -u https://target.com -json -o output.json

# Set concurrency
katana -u https://target.com -concurrency 10
```

### Flag Penting
| Flag | Fungsi |
|------|--------|
| `-u` | Single URL target |
| `-list` | List URL dari file |
| `-depth` | Kedalaman crawl |
| `-headless` | Mode headless (JavaScript rendering) |
| `-o` | Output ke file |
| `-json` | Output JSON |
| `-f` | Field output (url, qurl, endpoint) |
| `-extension-filter` | Filter ekstensi file |
| `-concurrency` | Jumlah concurrent request |
| `-timeout` | Timeout request |
| `-no-scope` | Crawl di luar scope domain |

### Contoh Output
```
https://target.com/login
https://target.com/api/v1/users
https://target.com/search?q=test
https://target.com/assets/app.js
```

---

## 5. gau (Get All URLs)

### Deskripsi
gau (Get All URLs) adalah tool untuk mengambil URL yang diketahui dari berbagai sumber pasif seperti Wayback Machine, OTX, URLScan, dan Common Crawl.

### Lokasi Binary
```
/usr/local/bin/gau
```

### Penggunaan Dasar
```bash
# Ambil semua URL domain
echo "target.com" | gau

# Termasuk subdomain
echo "target.com" | gau --subs

# Ambil dari provider tertentu
echo "target.com" | gau --providers wayback,otx

# Filter ekstensi
echo "target.com" | gau --blacklist jpg,png,gif,css

# Output ke file
echo "target.com" | gau --o urls.txt

# Set jumlah thread
echo "target.com" | gau --threads 10

# Ambil URL dari tahun tertentu
echo "target.com" | gau --from 2020 --to 2024

# JSON output
echo "target.com" | gau --json
```

### Flag Penting
| Flag | Fungsi |
|------|--------|
| `--subs` | Include subdomain |
| `--providers` | Pilih sumber data |
| `--blacklist` | Blacklist ekstensi |
| `--o` | Output ke file |
| `--threads` | Jumlah thread |
| `--from` | Tahun awal |
| `--to` | Tahun akhir |
| `--json` | Output JSON |
| `--retries` | Jumlah retry |
| `--timeout` | Timeout request |

### Sumber Data
- **Wayback Machine** — arsip internet
- **OTX** — AlienVault Open Threat Exchange
- **URLScan** — URLScan.io
- **Common Crawl** — crawl data publik

### Contoh Output
```
https://target.com/old-admin/
https://target.com/backup.zip
https://target.com/api/v1/users?id=1
https://target.com/.env
```

---

## 6. waybackurls

### Deskripsi
waybackurls adalah tool sederhana dari tomnomnom untuk mengambil semua URL yang pernah di-crawl dari Wayback Machine (archive.org) untuk domain tertentu.

### Lokasi Binary
```
/usr/local/bin/waybackurls
```

### Penggunaan Dasar
```bash
# Ambil URL dari stdin
echo "target.com" | waybackurls

# Dari multiple domain
cat domains.txt | waybackurls

# Output ke file
echo "target.com" | waybackurls > urls.txt

# Filter URL dengan parameter
echo "target.com" | waybackurls | grep "?"

# Filter URL dengan ekstensi tertentu
echo "target.com" | waybackurls | grep "\.php"

# Kombinasi dengan tool lain
echo "target.com" | waybackurls | httpx -silent
```

### Perbedaan dengan gau
| Aspek | waybackurls | gau |
|-------|-------------|-----|
| Sumber | Wayback Machine saja | Multiple (WB, OTX, URLScan, CC) |
| Kecepatan | Lebih cepat | Sedikit lebih lambat |
| Coverage | Terbatas | Lebih luas |
| Konfigurasi | Minimal | Lebih banyak opsi |

### Contoh Output
```
https://target.com/
http://target.com/login.php
https://target.com/admin/index.php?page=1
https://target.com/download.php?file=report.pdf
```

---

## 7. ffuf (Fuzz Faster U Fool)

### Deskripsi
ffuf adalah web fuzzer yang sangat cepat untuk directory discovery, parameter fuzzing, virtual host discovery, dan berbagai jenis fuzzing lainnya.

### Lokasi Binary
```
/usr/local/bin/ffuf
```

### Wordlist Default
```
/wordlists/common.txt
```

### Penggunaan Dasar
```bash
# Directory fuzzing
ffuf -u https://target.com/FUZZ -w /wordlists/common.txt

# Fuzzing dengan filter status code
ffuf -u https://target.com/FUZZ -w /wordlists/common.txt -mc 200,301,302

# Parameter fuzzing
ffuf -u "https://target.com/search?q=FUZZ" -w /wordlists/common.txt

# Virtual host fuzzing
ffuf -u https://target.com -H "Host: FUZZ.target.com" -w /wordlists/common.txt

# POST body fuzzing
ffuf -u https://target.com/login -X POST -d "username=FUZZ&password=test" -w users.txt

# Extension fuzzing
ffuf -u https://target.com/indexFUZZ -w /wordlists/extensions.txt

# Output JSON
ffuf -u https://target.com/FUZZ -w /wordlists/common.txt -json -o output.json

# Set jumlah thread
ffuf -u https://target.com/FUZZ -w /wordlists/common.txt -t 50

# Filter response size
ffuf -u https://target.com/FUZZ -w /wordlists/common.txt -fs 0

# Auto-calibrate filter
ffuf -u https://target.com/FUZZ -w /wordlists/common.txt -ac
```

### Flag Penting
| Flag | Fungsi |
|------|--------|
| `-u` | Target URL (gunakan FUZZ sebagai placeholder) |
| `-w` | Wordlist file |
| `-mc` | Match HTTP status code |
| `-fc` | Filter HTTP status code |
| `-ms` | Match response size |
| `-fs` | Filter response size |
| `-t` | Jumlah thread |
| `-X` | HTTP method |
| `-H` | Custom header |
| `-d` | POST data |
| `-json` | Output JSON |
| `-o` | Output ke file |
| `-ac` | Auto-calibrate filter |
| `-recursion` | Recursive fuzzing |
| `-e` | Extension list |

### Contoh Output
```
/admin          [Status: 200, Size: 1234, Words: 45]
/login          [Status: 200, Size: 890, Words: 32]
/backup         [Status: 301, Size: 0, Words: 0]
/.env           [Status: 200, Size: 156, Words: 12]
```

---

## 8. nuclei

### Deskripsi
Nuclei adalah vulnerability scanner berbasis template dari ProjectDiscovery. Menggunakan file YAML (template) untuk mendefinisikan cara mendeteksi vulnerability, misconfiguration, exposed panels, dan lainnya.

### Lokasi Binary
```
/usr/local/bin/nuclei       ← wrapper script (gunakan ini)
/usr/local/bin/nuclei-real  ← binary asli
```

### Templates Directory
```
/home/scanner/.nuclei-templates/   ← 12.958 templates (named volume)
```

### Struktur Templates
```
.nuclei-templates/
├── http/
│   ├── cves/           ← CVE spesifik
│   ├── technologies/   ← Deteksi teknologi
│   ├── exposures/      ← File/direktori exposed
│   ├── vulnerabilities/← Vulnerability umum
│   ├── misconfiguration/← Konfigurasi salah
│   ├── takeovers/      ← Subdomain takeover
│   ├── default-logins/ ← Credential default
│   └── fuzzing/        ← Fuzzing templates
├── network/            ← Network scanning
├── dns/                ← DNS checks
├── ssl/                ← SSL/TLS checks
└── workflows/          ← Multi-step workflows
```

### Penggunaan Dasar
```bash
# Scan dengan semua templates
nuclei -u https://target.com

# Scan dengan kategori tertentu
nuclei -u https://target.com -t /home/scanner/.nuclei-templates/http/technologies/

# Scan CVE saja
nuclei -u https://target.com -t /home/scanner/.nuclei-templates/http/cves/

# Scan dengan severity tertentu
nuclei -u https://target.com -severity critical,high

# Scan dari list target
nuclei -l targets.txt -t /home/scanner/.nuclei-templates/http/

# Output JSON
nuclei -u https://target.com -json-export output.json

# Output dengan stats
nuclei -u https://target.com -stats

# Silent mode
nuclei -u https://target.com -silent

# Scan dengan rate limit
nuclei -u https://target.com -rate-limit 100

# Scan dengan concurrency
nuclei -u https://target.com -concurrency 25
```

### Flag Penting
| Flag | Fungsi |
|------|--------|
| `-u` | Single URL target |
| `-l` | List target dari file |
| `-t` | Path template/direktori |
| `-severity` | Filter severity (critical/high/medium/low/info) |
| `-json-export` | Export hasil ke JSON |
| `-stats` | Tampilkan statistik scan |
| `-silent` | Hanya tampilkan temuan |
| `-rate-limit` | Rate limit request per detik |
| `-concurrency` | Jumlah template concurrent |
| `-timeout` | Timeout request |
| `-retries` | Jumlah retry |
| `-exclude-tags` | Exclude template berdasarkan tag |
| `-tags` | Filter template berdasarkan tag |

### Severity Level
| Level | Deskripsi |
|-------|-----------|
| `critical` | Vulnerability kritis, RCE, auth bypass |
| `high` | Vulnerability tinggi |
| `medium` | Vulnerability sedang |
| `low` | Vulnerability rendah |
| `info` | Informasi (deteksi teknologi, dll) |

### Contoh Output
```
[CVE-2021-44228] [http] [critical] https://target.com
[apache-detect] [http] [info] https://target.com ["Apache/2.4.7"]
[exposed-git] [http] [medium] https://target.com/.git/config
```

---

## 9. dalfox

### Deskripsi
Dalfox adalah XSS (Cross-Site Scripting) scanner yang powerful dari hahwul. Mendukung DOM-based XSS, reflected XSS, dan berbagai bypass technique.

### Lokasi Binary
```
/usr/local/bin/dalfox
```

### Penggunaan Dasar
```bash
# Scan single URL
dalfox url "https://target.com/?q=test"

# Scan dengan custom parameter
dalfox url "https://target.com/?q=test" -p q

# Scan dari file
dalfox file urls.txt

# Scan dengan pipe
cat urls.txt | dalfox pipe

# Scan dengan custom payload
dalfox url "https://target.com/?q=test" --custom-payload payloads.txt

# Scan dengan blind XSS
dalfox url "https://target.com/?q=test" -b "https://your-blind-xss.com"

# Output JSON
dalfox url "https://target.com/?q=test" --format json -o output.json

# Scan dengan cookie
dalfox url "https://target.com/?q=test" --cookie "session=abc123"

# Scan dengan header
dalfox url "https://target.com/?q=test" -H "Authorization: Bearer token"

# Worker count
dalfox url "https://target.com/?q=test" --worker 10

# Timeout
dalfox url "https://target.com/?q=test" --timeout 10
```

### Flag Penting
| Flag | Fungsi |
|------|--------|
| `-p` | Parameter yang di-test |
| `-b` | Blind XSS callback URL |
| `--cookie` | Cookie untuk authenticated scan |
| `-H` | Custom header |
| `--custom-payload` | Custom payload file |
| `--format` | Output format (plain/json) |
| `-o` | Output ke file |
| `--worker` | Jumlah worker |
| `--timeout` | Timeout request |
| `--skip-bav` | Skip BAV analysis |
| `--only-custom-payload` | Hanya pakai custom payload |
| `--mining-dict` | Mining parameter dari dictionary |

### Tipe XSS yang Dideteksi
- **Reflected XSS** — payload di-reflect di response
- **DOM-based XSS** — payload dieksekusi via DOM
- **Blind XSS** — payload dieksekusi di admin panel

### Contoh Output
```
[POC][G][BUILT-IN]: https://target.com/?q=<script>alert(1)</script>
[WEAK][XSS]: GET https://target.com/?q=test
```

---

## 10. sqlmap

### Deskripsi
sqlmap adalah tool otomatis untuk deteksi dan eksploitasi SQL injection. Mendukung berbagai database (MySQL, PostgreSQL, MSSQL, Oracle, SQLite, dll) dan berbagai teknik injection.

### Lokasi Binary
```
/usr/local/bin/sqlmap → /opt/sqlmap/sqlmap.py
```

### Penggunaan Dasar
```bash
# Test SQL injection basic
sqlmap -u "https://target.com/?id=1"

# Test dengan level dan risk tertentu
sqlmap -u "https://target.com/?id=1" --level=3 --risk=2

# Mode batch (tanpa interaksi)
sqlmap -u "https://target.com/?id=1" --batch

# Test POST parameter
sqlmap -u "https://target.com/login" --data="username=test&password=test"

# Test dengan cookie
sqlmap -u "https://target.com/?id=1" --cookie="session=abc123"

# Dump database
sqlmap -u "https://target.com/?id=1" --dbs

# Dump tabel dari database tertentu
sqlmap -u "https://target.com/?id=1" -D dbname --tables

# Dump data dari tabel
sqlmap -u "https://target.com/?id=1" -D dbname -T tablename --dump

# Deteksi DBMS saja
sqlmap -u "https://target.com/?id=1" --identify-waf

# Bypass WAF dengan tamper
sqlmap -u "https://target.com/?id=1" --tamper=space2comment

# Output ke file
sqlmap -u "https://target.com/?id=1" --output-dir=/home/scanner/.sqlmap/output
```

### Flag Penting
| Flag | Fungsi |
|------|--------|
| `-u` | Target URL |
| `--data` | POST data |
| `--cookie` | Cookie |
| `-p` | Parameter yang di-test |
| `--level` | Level test (1-5) |
| `--risk` | Risk level (1-3) |
| `--batch` | Tidak interaktif |
| `--dbs` | Enumerate database |
| `-D` | Pilih database |
| `--tables` | Enumerate tabel |
| `-T` | Pilih tabel |
| `--dump` | Dump data |
| `--tamper` | Script bypass WAF |
| `--dbms` | Force DBMS type |
| `--random-agent` | Random User-Agent |
| `--tor` | Gunakan Tor |

### Teknik Injection yang Didukung
| Teknik | Kode |
|--------|------|
| Boolean-based blind | B |
| Error-based | E |
| Union query | U |
| Stacked queries | S |
| Time-based blind | T |
| Inline queries | Q |

### Contoh Output
```
[INFO] GET parameter 'id' is 'AND boolean-based blind' injectable
[INFO] the back-end DBMS is MySQL
web server operating system: Linux Ubuntu
back-end DBMS: MySQL >= 5.0
```

---

## 11. mitmdump / mitmproxy

### Deskripsi
mitmproxy adalah interactive HTTPS proxy untuk intercepting, inspecting, modifying, dan replaying HTTP/HTTPS traffic. mitmdump adalah versi command-line dari mitmproxy, cocok untuk automation.

### Lokasi Binary
```
/usr/local/bin/mitmproxy  ← Interactive TUI
/usr/local/bin/mitmdump   ← Command-line (untuk automation)
/usr/local/bin/mitmweb    ← Web interface
```

### Versi
```
Mitmproxy: 12.2.2
Python: 3.13.5
```

### Penggunaan Dasar mitmdump
```bash
# Start proxy di port default (8080)
mitmdump

# Start proxy di port tertentu
mitmdump -p 9090

# Intercept dan simpan ke file
mitmdump -w traffic.dump

# Load dari file dump
mitmdump -r traffic.dump

# Filter request tertentu
mitmdump --flow-detail 3 -n -r traffic.dump

# Gunakan addon script
mitmdump -s script.py

# Transparent proxy mode
mitmdump --mode transparent

# Upstream proxy
mitmdump --mode upstream:http://proxy:8080

# SSL/TLS interception
mitmdump --ssl-insecure

# Set listen host
mitmdump --listen-host 0.0.0.0 -p 8080
```

### Penggunaan dalam Automation (Python Script)
```python
# addon.py - contoh script untuk mitmdump
from mitmproxy import http

def request(flow: http.HTTPFlow) -> None:
    # Log semua request
    print(f"Request: {flow.request.method} {flow.request.url}")

def response(flow: http.HTTPFlow) -> None:
    # Log semua response
    print(f"Response: {flow.response.status_code}")
    # Modifikasi response
    if "target.com" in flow.request.url:
        flow.response.headers["X-Custom"] = "intercepted"
```

```bash
# Jalankan dengan addon
mitmdump -s addon.py
```

### Flag Penting
| Flag | Fungsi |
|------|--------|
| `-p` | Listen port (default: 8080) |
| `-w` | Simpan traffic ke file |
| `-r` | Baca dari file dump |
| `-s` | Load addon script Python |
| `--mode` | Mode proxy (regular/transparent/upstream/socks5) |
| `--ssl-insecure` | Skip SSL verification |
| `--listen-host` | Listen host |
| `-n` | Tidak start proxy (hanya baca file) |
| `--flow-detail` | Level detail output (0-3) |

---

## Alur Penggunaan dalam APANT

### Typical Reconnaissance Flow
```
Target URL
    │
    ├── subfinder    → Enumerate subdomains
    │
    ├── httpx        → Probe semua subdomain (status, tech)
    │
    ├── katana       → Crawl semua URL dari target
    ├── gau          → Ambil URL historis
    ├── waybackurls  → Ambil URL dari Wayback Machine
    │
    └── Kumpulkan semua URL
```

### Typical Vulnerability Scanning Flow
```
URLs dari Reconnaissance
    │
    ├── nuclei       → Scan vulnerability & CVE
    │
    ├── nmap         → Port & service scanning
    │
    ├── ffuf         → Directory & parameter fuzzing
    │
    ├── dalfox       → XSS scanning (URL dengan parameter)
    │
    ├── sqlmap       → SQL injection testing
    │
    └── mitmdump     → Traffic interception & analysis
```

### Contoh Kombinasi Command
```bash
# 1. Subdomain → Probe → Crawl
subfinder -d target.com -silent | httpx -silent | katana -silent

# 2. URL Discovery
echo target.com | gau --blacklist jpg,png,gif > urls.txt
echo target.com | waybackurls >> urls.txt

# 3. Vulnerability Scan dari list URL
nuclei -l urls.txt -t /home/scanner/.nuclei-templates/http/ -severity critical,high

# 4. XSS Scan dari URL dengan parameter
cat urls.txt | grep "?" | dalfox pipe

# 5. SQLi Test
cat urls.txt | grep "?" | sqlmap -m - --batch --level=1 --risk=1
```

---

## Konfigurasi di Docker Container

### Environment Variables
```
SCANNER_PORT=8081
NMAP_BINARY=nmap
NUCLEI_TEMPLATES_PATH=/home/scanner/.nuclei-templates
HOME=/home/scanner
```

### Volumes
```
./data/wordlists:/wordlists:ro          ← Wordlist untuk ffuf
nuclei-templates:/home/scanner/.nuclei-templates  ← Nuclei templates
```

### tmpfs (Writable Directories)
```
/tmp
/home/scanner/.cache
/home/scanner/.config
/home/scanner/.sqlmap
/home/scanner/.local
/home/scanner/nuclei-reports
```

### Capabilities
```
CAP_NET_RAW    ← Untuk raw packet (nmap)
CAP_NET_ADMIN  ← Untuk network admin (nmap)
```

---

## Catatan Penting

1. **Gunakan hanya pada target yang diizinkan** — semua tool ini hanya boleh digunakan pada sistem yang Anda miliki atau telah mendapat izin eksplisit.

2. **nuclei wrapper** — gunakan command `nuclei` (bukan `nuclei-real`) untuk memastikan templates directory yang benar digunakan secara otomatis.

3. **sqlmap output** — hasil sqlmap tersimpan di `/home/scanner/.sqlmap` yang di-mount sebagai tmpfs dan akan hilang setelah container restart. Pastikan export hasil sebelum restart.

4. **Rate limiting** — selalu gunakan rate limiting yang wajar agar tidak membebani target dan menghindari deteksi.

5. **nuclei templates** — templates tersimpan di named volume Docker dan persist melewati restart container.
