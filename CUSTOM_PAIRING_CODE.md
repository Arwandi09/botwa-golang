# Custom Pairing Code (12345678)

Kode pairing WhatsApp di bot ini sudah di-custom jadi `12345678` (bisa kamu
ganti sendiri di `config.go`), bukan kode random bawaan WhatsApp — mirip
fitur di neoxr-bot.

## Ini SEKALI SAJA kamu lakukan (sebagai pemilik/maintainer repo)

Setelah ini selesai dan kamu commit + push ke GitHub, **orang lain yang
clone repo kamu TIDAK PERLU melakukan langkah apapun di bawah ini**.
Mereka tinggal clone, `go run .`, selesai — persis seperti bot Go biasa.

```bash
cd botwa                 # masuk ke folder project (yang ada go.mod)

# 1. Download semua dependency + buat folder vendor/
go mod vendor

# 2. Timpa 1 file whatsmeow dengan versi yang sudah dipatch
cp _patch_whatsmeow/go.mau.fi/whatsmeow/pair-code.go vendor/go.mau.fi/whatsmeow/pair-code.go

# 3. Tes jalanin dulu, pastikan kode yang muncul "12345678"
go run .

# 4. Kalau sudah OK, commit SEMUANYA termasuk folder vendor/ ke Git
git add vendor/ config.go main.go plugin/ go.mod go.sum
git commit -m "custom pairing code 12345678"
git push
```

Folder `_patch_whatsmeow/` cuma bahan mentah buat langkah 2 di atas —
setelah selesai, boleh dihapus dari repo kalau mau (nggak dipakai saat
runtime).

## Kenapa cukup sekali & orang lain nggak perlu ribet?

Karena `go.mod` project ini pakai `go 1.2x` (≥ 1.14), begitu folder
`vendor/` ada dan sudah ikut ke-commit di Git, **Go otomatis pakai isi
folder `vendor/` saat build** — tanpa perlu command khusus, tanpa flag
`-mod=vendor`, tanpa `go mod tidy` ulang. Jadi siapa pun yang:

```bash
git clone <link-repo-kamu>
cd botwa
go run .
```

...otomatis dapat versi whatsmeow yang sudah dipatch, tanpa tahu itu
patch, tanpa install apa-apa tambahan. Buat mereka rasanya sama persis
kayak clone project Go biasa.

## ⚠️ Satu hal yang perlu dihindari setelah ini

Jangan jalankan `go mod vendor` lagi di masa depan tanpa nge-copy ulang
`pair-code.go` yang sudah dipatch — soalnya `go mod vendor` akan menimpa
ulang semua isi `vendor/` termasuk file yang sudah kamu patch, balik ke
versi asli whatsmeow (kode random lagi). Kalau suatu saat nambah
dependency baru dan kepaksa jalanin `go mod vendor`, ulangi langkah 2 di
atas sebelum commit.

## Ganti kode pairing

Cukup edit satu baris di `config.go`:

```go
PairingCode = "12345678"   // ganti sesuka kamu, WAJIB persis 8 karakter
```

Berlaku otomatis untuk bot utama (`main.go`) dan plugin `.jadibot`.
