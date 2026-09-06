package plugin

import (
    "context"
    "fmt"
    "os"
    "path/filepath"
    "time"

    "go.mau.fi/whatsmeow"
    "go.mau.fi/whatsmeow/types/events"
)

const MediaCacheDir = "media_cache"

// MediaCacheTTL: setiap file di media_cache dihapus otomatis 48 jam
// setelah file itu disimpan (dihitung per file, bukan disamaratakan).
const MediaCacheTTL = 48 * time.Hour

func init() {
    // Buat folder cache jika belum ada
    os.MkdirAll(MediaCacheDir, 0755)
}

// Download dan cache media dari pesan
func DownloadAndCacheMedia(client *whatsmeow.Client, m *events.Message) string {
    ctx := context.Background()
    msg := m.Message

    var data []byte
    var err error
    var ext string
    var mediaType string

    // Image (✅ tambah ctx)
    if img := msg.GetImageMessage(); img != nil {
        data, err = client.Download(ctx, img)
        ext = ".jpg"
        mediaType = "image"
    } else if vid := msg.GetVideoMessage(); vid != nil {
        // Video
        data, err = client.Download(ctx, vid)
        ext = ".mp4"
        mediaType = "video"
    } else if aud := msg.GetAudioMessage(); aud != nil {
        // Audio
        data, err = client.Download(ctx, aud)
        ext = ".ogg"
        mediaType = "audio"
    } else if doc := msg.GetDocumentMessage(); doc != nil {
        // Document
        data, err = client.Download(ctx, doc)
        ext = filepath.Ext(doc.GetFileName())
        if ext == "" {
            ext = ".bin"
        }
        mediaType = "document"
    } else if sticker := msg.GetStickerMessage(); sticker != nil {
        // Sticker
        data, err = client.Download(ctx, sticker)
        ext = ".webp"
        mediaType = "sticker"
    } else {
        // Tidak ada media
        return ""
    }

    if err != nil {
        fmt.Println("❌ Error download media:", err)
        return ""
    }

    // Simpan file
    filename := fmt.Sprintf("%s_%s%s", m.Info.ID, mediaType, ext)
    filePath := filepath.Join(MediaCacheDir, filename)

    err = os.WriteFile(filePath, data, 0644)
    if err != nil {
        fmt.Println("❌ Error menyimpan file:", err)
        return ""
    }

    fmt.Printf("✅ Media disimpan: %s\n", filePath)

    // Jadwalkan penghapusan otomatis file ini persis 48 jam dari sekarang.
    scheduleMediaDeletion(filePath, MediaCacheTTL)

    return filePath
}

// scheduleMediaDeletion menghapus satu file media_cache setelah delay
// tertentu berlalu, dijalankan di background lewat timer (tanpa nge-block).
func scheduleMediaDeletion(filePath string, delay time.Duration) {
    time.AfterFunc(delay, func() {
        if err := os.Remove(filePath); err == nil {
            fmt.Printf("🗑️ Media cache terhapus otomatis (48 jam): %s\n", filePath)
        }
    })
}

// StartMediaCacheCleanup dipanggil sekali saat bot start. Fungsi ini:
//  1. Menjadwalkan ulang penghapusan untuk file yang sudah ada di
//     media_cache/ dari sebelum bot di-restart (supaya masa hidup 48 jam
//     tetap dihitung dari waktu file itu pertama disimpan, bukan reset).
//  2. Menjalankan sapu-bersih berkala sebagai jaring pengaman, untuk
//     menangkap file yang lolos (misal timer hilang karena proses sempat
//     mati/di-restart sebelum 48 jam tercapai).
func StartMediaCacheCleanup() {
    rescheduleExistingMediaCache()

    go func() {
        ticker := time.NewTicker(30 * time.Minute)
        defer ticker.Stop()
        for range ticker.C {
            CleanupMediaCache(MediaCacheTTL)
        }
    }()
}

// rescheduleExistingMediaCache memindai file yang sudah ada di media_cache/
// saat startup, lalu menjadwalkan penghapusannya berdasarkan sisa waktu
// dari 48 jam sejak file itu dimodifikasi terakhir (waktu simpan).
func rescheduleExistingMediaCache() {
    files, err := os.ReadDir(MediaCacheDir)
    if err != nil {
        return
    }

    now := time.Now()
    for _, file := range files {
        info, err := file.Info()
        if err != nil {
            continue
        }

        filePath := filepath.Join(MediaCacheDir, file.Name())
        age := now.Sub(info.ModTime())
        remaining := MediaCacheTTL - age

        if remaining <= 0 {
            os.Remove(filePath)
            fmt.Printf("🗑️ Hapus cache lama (sudah lewat 48 jam): %s\n", file.Name())
            continue
        }

        scheduleMediaDeletion(filePath, remaining)
    }
}

// CleanupMediaCache: sapu-bersih manual/berkala, menghapus semua file yang
// sudah lebih tua dari maxAge. Dipakai sebagai jaring pengaman oleh
// StartMediaCacheCleanup (lihat di atas).
func CleanupMediaCache(maxAge time.Duration) {
    files, err := os.ReadDir(MediaCacheDir)
    if err != nil {
        return
    }

    now := time.Now()
    for _, file := range files {
        info, err := file.Info()
        if err != nil {
            continue
        }

        age := now.Sub(info.ModTime())
        if age > maxAge {
            filePath := filepath.Join(MediaCacheDir, file.Name())
            os.Remove(filePath)
            fmt.Printf("🗑️ Hapus cache lama: %s\n", file.Name())
        }
    }
}