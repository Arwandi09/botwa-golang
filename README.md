# 🤖 botwa

**WhatsApp bot multi-fungsi berbasis Go + [whatsmeow](https://github.com/tulir/whatsmeow), dibangun dan dijalankan penuh di HP Android lewat Termux.**

Tidak perlu VPS, tidak perlu server berbayar — cukup HP Android, Termux, dan koneksi internet seadanya. `botwa` didesain ringan, dependency-minimal, dan pakai arsitektur plugin yang gampang dikembangkan.

---

## ✨ Fitur Utama

| Kategori | Command | Keterangan |
|---|---|---|
| ℹ️ Umum | `.menu` | Menampilkan daftar command, otomatis terkelompok per kategori |
| | `.ping` | Cek respon bot |
| | `.info` | Info singkat bot |
| | `.antidelete` | Anti-delete otomatis (background) — pesan yang dihapus pengirim tetap diteruskan lengkap dengan media aslinya |
| 📥 Download | `.ig <link>` | Download postingan Instagram (foto/video/reels/carousel), dengan **6 lapis fallback** biar tetap jalan walau satu metode diblokir |
| | `.igpp <username>` | Download foto profil Instagram |
| | `.tiktok <link>` | Download video TikTok tanpa watermark |
| | `.ytsearch <kata kunci>` | Cari video YouTube |
| | `.yta <link>` | Download audio YouTube (MP3) |
| | `.ytv <link>` | Download video YouTube (MP4 480p) |
| | `.rvo` (reply media) | Unduh ulang media *View Once* yang di-reply |
| 🎉 Grup & Fun | `.hidetag [teks]` | Mention seluruh member grup secara tersembunyi |
| | `.brat <teks>` | Bikin stiker teks bergaya *brat* |
| 🤖 Jadibot | `.jadibot [nomor]` | "Numpang" jadi bot — clone akun WhatsApp sendiri jadi bot terpisah lewat kode pairing |
| | `.aktifkanjadibot` | Aktifkan kembali seluruh sesi jadibot yang tersimpan |
| | `.stopjadibot <nomor>` | Hentikan satu sesi jadibot (file sesi tidak dihapus) |
| | `.stopsemuajadibot` | Hentikan seluruh sesi jadibot yang aktif |
| 🛠️ Owner & Maintenance | `.restart` | Restart bot tanpa bantuan process manager eksternal |
| | `.pluginadd/.pluginedit/.pluginrm` | Kelola file plugin langsung dari chat (balas kode → jadi file) |
| | `.mkdir` / `.mkfile` | Buat folder/file baru langsung dari chat |

Selain command di atas, ada juga fitur **diam-diam jalan di background**:

- 🕵️ **Reveal View Once via Reply** — kalau kamu me-reply media sekali-lihat, bot otomatis membuka & mengirim ulang isinya.
- 🗑️ **Anti-Delete** — semua pesan (termasuk media) disimpan sementara di cache, jadi kalau ada yang menghapus pesan, bot bisa menampilkan ulang isi aslinya lengkap dengan siapa pengirimnya.
- ♻️ **Media cache otomatis bersih** — setiap file media yang diunduh bot otomatis terhapus **48 jam** setelah disimpan (dihitung per file, bukan sekali sapu-bersih semua), jadi penyimpanan HP nggak penuh.
- 🔁 **Auto-reconnect & auto-activate jadibot** — begitu bot utama online (start awal ataupun reconnect), semua sesi jadibot yang pernah dibuat otomatis diaktifkan lagi setelah jeda 3 detik (biar tidak tabrakan dengan proses login bot utama).

---

## 🧠 Arsitektur

```
botwa/
├── main.go              # Entry point, koneksi WhatsApp, pairing code, event handler
├── handler.go           # Handler pesan utama (cache, anti-delete, view-once, command)
├── config.go            # Owner numbers, pairing code, prefix command
├── log/
│   └── raw.go           # Logger pesan masuk
└── plugin/
    ├── plugin.go         # Registry plugin (Register/Execute)
    ├── config.go         # Jembatan config dari main.go ke package plugin
    ├── cache.go          # Cache pesan (untuk anti-delete)
    ├── mediadownloader.go# Download & auto-cleanup media_cache (TTL 48 jam/file)
    ├── antidelete.go     # Logic anti-delete + forward pesan asli
    ├── antiviewonce.go   # Reveal view-once via reply
    ├── jadibot.go        # Multi-session clone bot (jadibot)
    ├── aktifkanjadibot.go
    ├── stopjadibot.go
    ├── stopsemuajadibot.go
    ├── instagram.go      # Download IG (yt-dlp + gallery-dl + embed + scrape fallback)
    ├── tiktok.go
    ├── youtube.go
    ├── rvo.go
    ├── hidetag.go
    ├── brat.go
    ├── filemgr.go        # Plugin/file manager lewat chat (owner-only)
    ├── restart.go
    ├── menu.go
    ├── ping.go
    └── info.go
```

### Cara kerja plugin

Setiap plugin **self-register** lewat `init()`:

```go
func init() {
    Register(Plugin{
        Command: "ping",
        Desc:    "Tes respon bot",
        Run: func(client *whatsmeow.Client, m *events.Message, args []string) {
            // ...
        },
    })
}
```

Tinggal taruh file baru di folder `plugin/`, isi `init()` seperti di atas, dan otomatis muncul di `.menu` serta bisa langsung dipanggil — tanpa perlu edit file lain. Bahkan bisa dilakukan **langsung dari chat WhatsApp** lewat `.pluginadd`.

---

## ⚙️ Instalasi (Termux / Android)

### 1. Install dependency dasar

```bash
pkg update && pkg upgrade -y
pkg install golang git ffmpeg python -y
pip install yt-dlp gallery-dl --break-system-packages
```

### 2. Clone / extract project

```bash
cd ~
# extract botwa1.zip ke sini, lalu masuk ke foldernya
cd botwa1
```

### 3. Build

```bash
go build -o botwa .
```

> Project ini sudah menyertakan folder `vendor/`, jadi build tetap bisa jalan walau koneksi internet lagi lambat/putus — tidak perlu `go mod download` ulang.

### 4. Konfigurasi

Edit `config.go`:

```go
const (
    PairingNumber = "628xxxxxxxxxx" // nomor WA bot utama
    PairingCode   = "12345678"      // 8 karakter, atau "" untuk kode random
)

var OwnerNumbers = []string{
    "628xxxxxxxxxx", // nomor owner
}
```

### 5. Jalankan

```bash
./botwa
```

Saat pertama kali jalan, bot akan menampilkan kode pairing di terminal — buka **WhatsApp → Perangkat Tertaut → Gunakan Kode**, masukkan kodenya, dan bot langsung online.

---

## 📌 Catatan Penting

- **Session tidak boleh dihapus.** File `session.db` dan folder `session_jadibot/` menyimpan sesi login WhatsApp — hapus file ini = harus pairing ulang.
- **Instagram download** paling stabil kalau ada `ig_cookies.txt` (format Netscape, hasil export dari browser saat login ke instagram.com). Tanpa cookies, bot tetap jalan lewat fallback (yt-dlp → gallery-dl → embed page → scrape langsung) tapi tidak sekonsisten dengan cookies.
- **Owner check** memakai `IsFromMe` + daftar `OwnerNumbers`. Karena format LID WhatsApp kadang tidak sama persis dengan nomor telepon biasa, pastikan nomor di `OwnerNumbers` sesuai format yang benar-benar terkirim di event pesan.
- **Media cache** (`media_cache/`) otomatis bersih sendiri, tidak perlu dibersihkan manual — tiap file punya umur maksimal 48 jam sejak disimpan.
- Konstanta `TargetNumber` di `antidelete.go` dan target JID di `antiviewonce.go` saat ini di-hardcode ke satu nomor tujuan notifikasi — ganti sesuai kebutuhanmu kalau mau dipakai orang lain.

---

## 🧩 Tech Stack

- **Bahasa:** Go 1.26
- **Library WhatsApp:** [whatsmeow](https://github.com/tulir/whatsmeow) (multi-device, sqlite-backed session store)
- **Database sesi:** SQLite (`mattn/go-sqlite3`)
- **Download media eksternal:** `yt-dlp`, `gallery-dl`, `ffmpeg`
- **Platform target:** Android via Termux (juga jalan normal di Linux/VPS)

---

## 🗺️ Roadmap / Ide Pengembangan

- [ ] Auto-update `yt-dlp` berkala biar nggak ketinggalan patch dari perubahan API platform
- [ ] Dashboard sederhana buat monitor sesi jadibot yang aktif
- [ ] Rate limiter per user/grup buat command download

---

## ⚠️ Disclaimer

Project ini dibuat untuk keperluan belajar & personal use. Gunakan fitur download media (Instagram/TikTok/YouTube) sesuai dengan Ketentuan Layanan platform terkait dan hak cipta konten yang bersangkutan. Pengembang tidak bertanggung jawab atas penyalahgunaan bot ini.

---

<p align="center">Dibangun dengan ❤️ sepenuhnya dari HP Android via Termux.</p>
go run main.go
