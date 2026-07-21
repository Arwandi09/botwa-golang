<div align="center">

# 🤖 botwa-golang

**Bot WhatsApp berbasis Go, ditenagai [whatsmeow](https://github.com/tulir/whatsmeow)**

Ringan • Modular • Tanpa scan QR • Multi-session (Jadibot)

[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?style=flat&logo=go&logoColor=white)](https://go.dev)
[![whatsmeow](https://img.shields.io/badge/library-whatsmeow-25D366?style=flat&logo=whatsapp&logoColor=white)](https://github.com/tulir/whatsmeow)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](#lisensi)

</div>

---

## ✨ Kenapa botwa-golang?

Kebanyakan bot WhatsApp open source dibangun pakai Node.js (Baileys). botwa-golang mengambil pendekatan berbeda: **native Go**, satu binary, minim dependency, dan ringan dijalankan bahkan di HP lewat Termux — tanpa perlu Node, npm, atau puluhan `node_modules`.

- 🔑 **Login tanpa QR** — pairing lewat kode, termasuk **kode pairing custom** yang bisa kamu tentukan sendiri
- 🧩 **Struktur plugin modular** — tambah command baru tinggal bikin 1 file di `plugin/`
- 👥 **Jadibot** — orang lain bisa numpang jadi bot pakai akun WA mereka sendiri, tanpa ganggu bot utama
- 💾 **Sesi otomatis tersimpan** via SQLite — sekali pairing, langsung connect terus
- 🪶 **Ringan** — cocok jalan 24/7 di VPS kecil maupun HP Android (Termux)

---

## 📦 Fitur

| Command | Deskripsi |
|---|---|
| `.menu` | Menampilkan daftar menu |
| `.ping` | Tes respon bot |
| `.info` | Info bot |
| `.brat <teks>` | Bikin stiker teks bergaya brat |
| `.hidetag <teks>` | Mention semua member grup secara tersembunyi |
| `.rvo` (reply media) | Download & kirim ulang media View Once |
| `.ytsearch <query>` | Cari video YouTube |
| `.yta <url>` | Download audio YouTube (MP3) |
| `.ytv <url>` | Download video YouTube (MP4 480p) |
| `.jadibot <nomor>` | Numpang jadi bot pakai akun WA sendiri |
| `.stopjadibot <nomor>` | *(Owner)* Hentikan satu sesi jadibot |
| `.stopsemuajadibot` | *(Owner)* Hentikan semua sesi jadibot aktif |
| `.aktifkanjadibot` | *(Owner)* Nyalakan ulang semua sesi jadibot tersimpan |

Plus sistem **anti-delete** dan **anti-viewonce otomatis** yang jalan di background tanpa command.

> Semua command mengenali banyak prefix: `. : ! ? / * , " ' & # >` — dan sekarang tetap merespon walau ada spasi setelah prefix (`. menu` sama validnya dengan `.menu`).

---

## 🚀 Instalasi

```bash
git clone https://github.com/USERNAME/botwa-golang.git
cd botwa-golang
go run .
```

Saat pertama kali jalan, bot akan menampilkan kode pairing di terminal:

```
================================
 PAIRING CODE : 12345678
================================
WhatsApp → Perangkat tertaut → Gunakan kode
```

Buka **WhatsApp → Perangkat Tertaut → Tautkan Perangkat → Gunakan nomor telepon**, lalu masukkan kode di atas.

> Kode pairing bisa kamu custom sendiri lewat `config.go` — lihat bagian [Konfigurasi](#%EF%B8%8F-konfigurasi).

### Menjalankan di Termux (Android)

```bash
pkg install golang git -y
git clone https://github.com/USERNAME/botwa-golang.git
cd botwa-golang
go run .
```

---

## ⚙️ Konfigurasi

Semua pengaturan ada di `config.go`:

```go
const (
    PairingNumber = "6285xxxxxxxxx" // nomor WA bot, format internasional tanpa +
    PairingCode   = "12345678"      // kode pairing custom, WAJIB persis 8 karakter
)

var OwnerNumbers = []string{
    "6285xxxxxxxxx", // nomor owner, bisa lebih dari satu
}
```

Kosongkan `PairingCode = ""` untuk kembali memakai kode random bawaan WhatsApp.

---

## 🧩 Menambah Plugin Sendiri

Setiap plugin cukup satu file di folder `plugin/`, didaftarkan lewat `init()`:

```go
package plugin

import (
    "go.mau.fi/whatsmeow"
    "go.mau.fi/whatsmeow/types/events"
)

func init() {
    Register(Plugin{
        Command: "halo",
        Desc:    "Contoh plugin sederhana",
        Run: func(client *whatsmeow.Client, m *events.Message, args []string) {
            reply(client, m, "Halo juga! 👋")
        },
    })
}
```

Restart bot, dan command `.halo` langsung aktif — tidak perlu daftar manual di tempat lain.

---

## 🗂️ Struktur Project

```
botwa-golang/
├── main.go            # Entry point & koneksi WhatsApp
├── config.go           # Konfigurasi (nomor, kode pairing, prefix)
├── handler.go           # Router pesan masuk
├── plugin/               # Semua command & fitur bot
│   ├── jadibot.go          # Sistem multi-session jadibot
│   ├── antidelete.go
│   ├── antiviewonce.go
│   └── ...
└── log/                   # Logger pesan
```

---

## 🛠️ Teknologi

- [Go](https://go.dev) — bahasa utama
- [whatsmeow](https://github.com/tulir/whatsmeow) — WhatsApp multi-device API
- SQLite — penyimpanan sesi

---

## ⚠️ Disclaimer

Project ini memakai library unofficial (reverse-engineered) untuk WhatsApp Web API. Gunakan dengan bijak, sesuai Ketentuan Layanan WhatsApp, dan bukan untuk spam atau aktivitas yang melanggar aturan.

---

## 📄 Lisensi

MIT — bebas dipakai, dimodifikasi, dan disebarkan. Kontribusi & pull request selalu diterima 🙌

---

<div align="center">

Dibuat dengan ☕ dan sedikit trial-error di Termux

</div>
go run main.go
